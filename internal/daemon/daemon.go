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

	"wildgecu/agent"
	"wildgecu/cron"
	"wildgecu/homer"
	"wildgecu/provider/gemini"
	"wildgecu/x/config"
	"wildgecu/chat/telegram"
)

// Config holds daemon configuration.
type Config struct {
	Version       string
	APIKey        string
	Model         string
	TelegramToken string
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

	// --- Session manager initialization ---
	var sm *SessionManager
	if cfg.APIKey != "" {
		var err error
		sm, err = initSessionManager(ctx, cfg, logger)
		if err != nil {
			logger.Warn("session manager disabled", "error", err)
		} else {
			srv.SetSessions(sm)
			logger.Info("session manager ready")
		}
	}

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

	// --- Cron scheduler initialization ---
	if cfg.APIKey != "" {
		globalHome, err := config.GlobalHome()
		if err != nil {
			return fmt.Errorf("global home: %w", err)
		}

		cronsHomer, err := homer.New(filepath.Join(globalHome, "crons"))
		if err != nil {
			return fmt.Errorf("crons homer: %w", err)
		}

		resultsHomer, err := homer.New(filepath.Join(globalHome, "cron-results"))
		if err != nil {
			return fmt.Errorf("cron-results homer: %w", err)
		}

		p, err := gemini.New(ctx, cfg.APIKey, cfg.Model)
		if err != nil {
			return fmt.Errorf("gemini provider: %w", err)
		}

		execCfg := &cron.ExecutorConfig{
			Provider: p,
			Results:  resultsHomer,
			Logger:   logger,
		}

		scheduler, err = cron.NewScheduler(cronsHomer, execCfg, logger)
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
		tgBridge, err := telegram.New(cfg.TelegramToken, sm)
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
func initSessionManager(ctx context.Context, cfg Config, logger *slog.Logger) (*SessionManager, error) {
	globalHome, err := config.GlobalHome()
	if err != nil {
		return nil, fmt.Errorf("global home: %w", err)
	}

	home, err := homer.New(globalHome)
	if err != nil {
		return nil, fmt.Errorf("home homer: %w", err)
	}

	p, err := gemini.New(ctx, cfg.APIKey, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("gemini provider: %w", err)
	}

	skillsHome, err := homer.New(filepath.Join(globalHome, "skills"))
	if err != nil {
		return nil, fmt.Errorf("skills homer: %w", err)
	}

	agentCfg := agent.Config{
		Provider:   p,
		Home:       home,
		Workspace:  home, // daemon uses global home as workspace
		SkillsHome: skillsHome,
		HomeDir:    globalHome,
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
