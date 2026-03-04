package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// HealthCheck defines a component that can be monitored by the watchdog.
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) error
	Restart(ctx context.Context) error
}

// Watchdog periodically runs health checks and restarts components after
// consecutive failures.
type Watchdog struct {
	checks   []HealthCheck
	failures map[string]int
	mu       sync.Mutex
	logger   *slog.Logger
}

// NewWatchdog creates a new Watchdog with the given health checks.
func NewWatchdog(logger *slog.Logger, checks ...HealthCheck) *Watchdog {
	return &Watchdog{
		checks:   checks,
		failures: make(map[string]int),
		logger:   logger,
	}
}

// Run starts the watchdog loop, checking every 30 seconds until ctx is cancelled.
func (w *Watchdog) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runChecks(ctx)
		}
	}
}

func (w *Watchdog) runChecks(ctx context.Context) {
	for _, c := range w.checks {
		if err := c.Check(ctx); err != nil {
			w.mu.Lock()
			w.failures[c.Name()]++
			count := w.failures[c.Name()]
			w.mu.Unlock()

			w.logger.Warn("health check failed", "check", c.Name(), "failures", count, "error", err)

			if count >= 3 {
				w.logger.Info("restarting component", "check", c.Name())
				if restartErr := c.Restart(ctx); restartErr != nil {
					w.logger.Error("restart failed", "check", c.Name(), "error", restartErr)
				} else {
					w.mu.Lock()
					w.failures[c.Name()] = 0
					w.mu.Unlock()
				}
			}
		} else {
			w.mu.Lock()
			w.failures[c.Name()] = 0
			w.mu.Unlock()
		}
	}
}

// Status returns a copy of the current failure counts.
func (w *Watchdog) Status() map[string]int {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make(map[string]int, len(w.failures))
	for k, v := range w.failures {
		out[k] = v
	}
	return out
}
