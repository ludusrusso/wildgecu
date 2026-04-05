package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"wildgecu/pkg/agent"
	"wildgecu/pkg/chat/telegram"
	"wildgecu/pkg/cron"
	"wildgecu/pkg/home"
	"wildgecu/pkg/provider/factory"
	"wildgecu/pkg/telegram/auth"
	"wildgecu/x/config"
)

// Config holds daemon configuration.
type Config struct {
	Version       string
	Provider      string // "gemini", "openai", or "ollama"
	APIKey        string
	Model         string
	TelegramToken string
	GoogleSearch  bool
	OllamaURL     string
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
	if cfg.APIKey == "" && cfg.Provider != "ollama" {
		return fmt.Errorf("API key not set for provider %q; configure it in your config file or environment", cfg.Provider)
	}

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
	if h != nil {
		p, err := factory.New(ctx, cfg.factoryConfig())
		if err != nil {
			return fmt.Errorf("provider: %w", err)
		}

		execCfg := &cron.ExecutorConfig{
			Provider: p,
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
		logger.Info("cron scheduler disabled (no API key configured)")
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
		tgBridge, err := telegram.New(cfg.TelegramToken, sm, tgAuth)
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

// factoryConfig returns a factory.Config from the daemon Config.
func (c Config) factoryConfig() factory.Config {
	return factory.Config{
		Provider:     c.Provider,
		Model:        c.Model,
		APIKey:       c.APIKey,
		GoogleSearch: c.GoogleSearch,
		OllamaURL:    c.OllamaURL,
	}
}

// initSessionManager creates the agent config and initializes the session manager.
func initSessionManager(ctx context.Context, cfg Config, h *home.Home, tgAuth *auth.Store, _ *slog.Logger) (*SessionManager, error) {
	p, err := factory.New(ctx, cfg.factoryConfig())
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
