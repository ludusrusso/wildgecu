package cron

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ludusrusso/wildgecu/pkg/provider"
)

// ExecutorConfig holds the dependencies for executing a cron job.
type ExecutorConfig struct {
	Provider provider.Provider
	Results  string // path to cron-results directory
	Logger   *slog.Logger
}

// Execute runs a single cron job: calls the provider with the prompt
// and writes the result to the results home.
func Execute(ctx context.Context, cfg *ExecutorConfig, job *CronJob) {
	cfg.Logger.Info("executing cron job", "name", job.Name, "timeout", job.Timeout)

	if job.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, job.Timeout)
		defer cancel()
	}

	resp, err := cfg.Provider.Generate(ctx, &provider.GenerateParams{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: job.Prompt},
		},
	})
	if err != nil {
		if job.Timeout > 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			cfg.Logger.Error("cron job timed out", "name", job.Name, "timeout", job.Timeout)
			writeResult(cfg, job.Name, fmt.Sprintf("Timed out after %s.\n", job.Timeout))
			return
		}
		cfg.Logger.Error("cron job failed", "name", job.Name, "error", err)
		return
	}

	if filename := writeResult(cfg, job.Name, resp.Message.Content); filename != "" {
		cfg.Logger.Info("cron job completed", "name", job.Name, "file", filename)
	}
}

func writeResult(cfg *ExecutorConfig, name, content string) string {
	ts := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.md", name, ts)
	if err := os.MkdirAll(cfg.Results, 0o755); err != nil {
		cfg.Logger.Error("failed to create results dir", "name", name, "error", err)
		return ""
	}
	if err := os.WriteFile(filepath.Join(cfg.Results, filename), []byte(content), 0o644); err != nil {
		cfg.Logger.Error("failed to write cron result", "name", name, "error", err)
		return ""
	}
	return filename
}
