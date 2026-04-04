package cron

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"wildgecu/pkg/provider"
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
	cfg.Logger.Info("executing cron job", "name", job.Name)

	resp, err := cfg.Provider.Generate(ctx, &provider.GenerateParams{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: job.Prompt},
		},
	})
	if err != nil {
		cfg.Logger.Error("cron job failed", "name", job.Name, "error", err)
		return
	}

	ts := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.md", job.Name, ts)

	if err := os.MkdirAll(cfg.Results, 0o755); err != nil {
		cfg.Logger.Error("failed to create results dir", "name", job.Name, "error", err)
		return
	}
	if err := os.WriteFile(filepath.Join(cfg.Results, filename), []byte(resp.Message.Content), 0o644); err != nil {
		cfg.Logger.Error("failed to write cron result", "name", job.Name, "error", err)
	}

	cfg.Logger.Info("cron job completed", "name", job.Name, "file", filename)
}
