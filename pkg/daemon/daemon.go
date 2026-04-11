package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"wildgecu/pkg/agent"
	"wildgecu/pkg/chat/telegram"
	"wildgecu/pkg/command"
	"wildgecu/pkg/cron"
	"wildgecu/pkg/home"
	"wildgecu/pkg/telegram/auth"
	"wildgecu/x/config"
	"wildgecu/x/container"
)

// Config holds daemon configuration.
type Config struct {
	Version       string
	DefaultModel  string
	TelegramToken string
	Container     *container.Container
}

// Run is the main daemon loop. It manages the PID file, socket server, watchdog,
// and signal handling. It blocks until the context is cancelled or a shutdown
// signal is received.
func Run(ctx context.Context, cfg Config) error {
	logger := slog.Default()

	if err := WritePID(); err != nil {
		return fmt.Errorf("write pid: %w", err)
	}
	defer RemovePID()

	srv, err := NewSocketServer(logger)
	if err != nil {
		return fmt.Errorf("socket server: %w", err)
	}
	defer srv.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	startTime := time.Now()
	wd := NewWatchdog(logger)

	// --- Home directory initialization ---
	globalHome, err := config.GlobalHome()
	if err != nil {
		return fmt.Errorf("global home: %w", err)
	}

	h, err := home.New(globalHome)
	if err != nil {
		return fmt.Errorf("home: %w", err)
	}

	// --- Telegram auth store ---
	var tgAuth *auth.Store
	if cfg.TelegramToken != "" {
		tgAuthPath := filepath.Join(globalHome, "telegram.json")
		tgAuth, err = auth.New(tgAuthPath)
		if err != nil {
			return fmt.Errorf("telegram auth: %w", err)
		}
	}

	// --- Session manager initialization ---
	sm, err := initSessionManager(ctx, cfg, h, tgAuth, logger)
	if err != nil {
		return fmt.Errorf("cannot start daemon: %w", err)
	}
	srv.SetSessions(sm)
	logger.Info("session manager ready")

	// --- Slash command registry ---
	cmdRegistry := command.NewRegistry(h.SkillsDir())
	helpCmd := command.NewHelpCommand(cmdRegistry)
	cmdRegistry.Register(helpCmd)
	cleanCmd := command.NewCleanCommand(func(ctx context.Context, id string) (string, error) {
		return sm.ResetSession(ctx, id)
	})
	cmdRegistry.Register(cleanCmd)
	statusCmd := command.NewStatusCommand(func(_ context.Context, id string) (command.StatusInfo, error) {
		sess := sm.Get(id)
		if sess == nil {
			return command.StatusInfo{}, fmt.Errorf("session not found: %s", id)
		}
		var toolCalls int
		uniqueSkills := make(map[string]struct{})
		for _, msg := range sess.Messages {
			toolCalls += len(msg.ToolCalls)
			for _, tc := range msg.ToolCalls {
				if tc.Name == "read_skill" {
					if name, ok := tc.Args["name"].(string); ok {
						uniqueSkills[name] = struct{}{}
					}
				}
			}
		}
		providerName, modelName, _ := strings.Cut(cfg.DefaultModel, "/")
		return command.StatusInfo{
			SessionID:    sess.ID,
			MessageCount: len(sess.Messages),
			ToolCalls:    toolCalls,
			SkillsLoaded: len(uniqueSkills),
			Provider:     providerName,
			Model:        modelName,
			Uptime:       time.Since(sess.createdAt),
		}, nil
	})
	cmdRegistry.Register(statusCmd)
	srv.SetCommands(cmdRegistry)

	// --- Cron scheduler (declared early so status handler can access it) ---

	var scheduler *cron.Scheduler

	// --- Register command handlers ---

	srv.Handle("ping", func(r *Request) (*Response, error) {
		return &Response{OK: true, Payload: "pong"}, nil
	})

	srv.Handle("status", func(r *Request) (*Response, error) {
		pid, _ := ReadPID()
		payload := map[string]any{
			"pid":      pid,
			"uptime":   time.Since(startTime).String(),
			"version":  cfg.Version,
			"watchdog": wd.Status(),
		}
		if scheduler != nil {
			payload["crons"] = scheduler.ListJobs()
		}
		return &Response{
			OK:      true,
			Payload: payload,
		}, nil
	})

	srv.Handle("stop", func(r *Request) (*Response, error) {
		// Delay cancel so the response can be sent back to the client.
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		return &Response{OK: true, Payload: "shutting down"}, nil
	})

	srv.Handle("update", func(r *Request) (*Response, error) {
		url, _ := r.Args["url"].(string)
		if url == "" {
			return &Response{OK: false, Error: "missing url argument"}, nil
		}
		go func() {
			if err := SelfUpdate(url); err != nil {
				logger.Error("self-update failed", "error", err)
			}
		}()
		return &Response{OK: true, Payload: "update started"}, nil
	})

	if tgAuth != nil {
		srv.Handle("approve-telegram", func(r *Request) (*Response, error) {
			otp, _ := r.Args["otp"].(string)
			if otp == "" {
				return &Response{OK: false, Error: "missing otp argument"}, nil
			}
			userID, err := tgAuth.ApproveByOTP(otp)
			if err != nil {
				return &Response{OK: false, Error: "Invalid OTP"}, nil
			}
			return &Response{
				OK:      true,
				Payload: fmt.Sprintf("User %d approved", userID),
			}, nil
		})
	}

	// --- Cron scheduler initialization ---
	if h != nil && cfg.Container != nil {
		cronProvider, err := cfg.Container.Get(ctx, cfg.DefaultModel)
		if err != nil {
			return fmt.Errorf("cron provider: %w", err)
		}

		execCfg := &cron.ExecutorConfig{
			Provider: cronProvider,
			Results:  h.CronResultsDir(),
			Logger:   logger,
		}

		scheduler, err = cron.NewScheduler(h.CronsDir(), execCfg, logger)
		if err != nil {
			return fmt.Errorf("cron scheduler: %w", err)
		}

		if err := scheduler.LoadAndStart(ctx); err != nil {
			return fmt.Errorf("cron start: %w", err)
		}

		srv.Handle("cron-reload", func(r *Request) (*Response, error) {
			if err := scheduler.Reload(ctx); err != nil {
				return &Response{OK: false, Error: err.Error()}, nil
			}
			return &Response{OK: true, Payload: "cron jobs reloaded"}, nil
		})
	} else {
		logger.Info("cron scheduler disabled (no provider configured)")
	}

	// --- Start socket server and watchdog ---

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.Serve(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		wd.Run(ctx)
	}()

	// --- Telegram bot ---
	if cfg.TelegramToken != "" && sm != nil {
		tgBridge, err := telegram.New(cfg.TelegramToken, sm, tgAuth, cmdRegistry)
		if err != nil {
			logger.Warn("telegram bot disabled", "error", err)
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := tgBridge.Run(ctx); err != nil && ctx.Err() == nil {
					logger.Error("telegram bot error", "error", err)
				}
			}()
			logger.Info("telegram bot started")
		}
	}

	logger.Info("daemon started", "pid", os.Getpid(), "version", cfg.Version)

	// --- Signal handling ---

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1)

	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		logger.Info("received signal", "signal", sig)
		switch sig {
		case syscall.SIGUSR1:
			// Re-exec: cancel, wait for goroutines, then exec the new binary.
			cancel()
			waitWithTimeout(&wg, 10*time.Second)
			srv.Close()

			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve executable: %w", err)
			}
			return syscall.Exec(exe, os.Args, os.Environ())
		default:
			cancel()
		}
	}

	// --- Graceful shutdown ---

	if scheduler != nil {
		if err := scheduler.Stop(); err != nil {
			logger.Error("cron scheduler stop error", "error", err)
		}
	}
	waitWithTimeout(&wg, 10*time.Second)
	logger.Info("daemon stopped")
	return nil
}

// initSessionManager creates the agent config and initializes the session manager.
func initSessionManager(ctx context.Context, cfg Config, h *home.Home, tgAuth *auth.Store, _ *slog.Logger) (*SessionManager, error) {
	p, err := cfg.Container.Get(ctx, cfg.DefaultModel)
	if err != nil {
		return nil, fmt.Errorf("provider: %w", err)
	}

	agentCfg := agent.Config{
		Provider:     p,
		Home:         h,
		Workspace:    h, // daemon uses global home as workspace
		TelegramAuth: tgAuth,
	}

	return NewSessionManager(ctx, agentCfg)
}

func waitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}
