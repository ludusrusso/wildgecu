package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Config holds daemon configuration.
type Config struct {
	Version string
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

	// --- Register command handlers ---

	srv.Handle("ping", func(r *Request) (*Response, error) {
		return &Response{OK: true, Payload: "pong"}, nil
	})

	srv.Handle("status", func(r *Request) (*Response, error) {
		pid, _ := ReadPID()
		return &Response{
			OK: true,
			Payload: map[string]any{
				"pid":      pid,
				"uptime":   time.Since(startTime).String(),
				"version":  cfg.Version,
				"watchdog": wd.Status(),
			},
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

	waitWithTimeout(&wg, 10*time.Second)
	logger.Info("daemon stopped")
	return nil
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
