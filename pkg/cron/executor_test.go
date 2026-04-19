package cron

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ludusrusso/wildgecu/pkg/provider"
)

// mockProvider returns a fixed response for testing.
type mockProvider struct {
	response string
}

func (m *mockProvider) Generate(_ context.Context, _ *provider.GenerateParams) (*provider.Response, error) {
	return &provider.Response{
		Message: provider.Message{
			Role:    provider.RoleModel,
			Content: m.response,
		},
	}, nil
}

func TestExecute(t *testing.T) {
	resultsDir := t.TempDir()
	cfg := &ExecutorConfig{
		Provider: &mockProvider{response: "Here is your summary"},
		Results:  resultsDir,
		Logger:   slog.Default(),
	}

	job := &CronJob{
		Name:     "daily-summary",
		Schedule: "0 9 * * *",
		Prompt:   "Summarize my day",
	}

	Execute(context.Background(), cfg, job)

	matches, err := filepath.Glob(filepath.Join(resultsDir, "daily-summary-*.md"))
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 result file, got %d", len(matches))
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "Here is your summary" {
		t.Errorf("expected result content, got %q", data)
	}

	filename := filepath.Base(matches[0])
	if !strings.HasPrefix(filename, "daily-summary-") {
		t.Errorf("expected filename to start with daily-summary-, got %q", filename)
	}
	if !strings.HasSuffix(filename, ".md") {
		t.Errorf("expected filename to end with .md, got %q", filename)
	}
}

type blockingProvider struct{}

func (p *blockingProvider) Generate(ctx context.Context, _ *provider.GenerateParams) (*provider.Response, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type recordingProvider struct {
	hadDeadline bool
	response    string
}

func (p *recordingProvider) Generate(ctx context.Context, _ *provider.GenerateParams) (*provider.Response, error) {
	_, p.hadDeadline = ctx.Deadline()
	return &provider.Response{
		Message: provider.Message{Role: provider.RoleModel, Content: p.response},
	}, nil
}

func TestExecuteTimeout(t *testing.T) {
	t.Run("cancels provider when deadline fires and records timeout", func(t *testing.T) {
		resultsDir := t.TempDir()
		cfg := &ExecutorConfig{
			Provider: &blockingProvider{},
			Results:  resultsDir,
			Logger:   slog.Default(),
		}
		job := &CronJob{
			Name:     "slow",
			Schedule: "0 9 * * *",
			Timeout:  20 * time.Millisecond,
			Prompt:   "hang",
		}

		start := time.Now()
		Execute(context.Background(), cfg, job)
		elapsed := time.Since(start)

		if elapsed > 2*time.Second {
			t.Fatalf("Execute did not respect timeout; ran for %s", elapsed)
		}

		matches, err := filepath.Glob(filepath.Join(resultsDir, "slow-*.md"))
		if err != nil {
			t.Fatalf("Glob failed: %v", err)
		}
		if len(matches) != 1 {
			t.Fatalf("expected 1 result file, got %d", len(matches))
		}
		data, _ := os.ReadFile(matches[0])
		if !strings.Contains(string(data), "Timed out") {
			t.Errorf("expected timeout marker in result, got %q", data)
		}
	})

	t.Run("no timeout means no deadline", func(t *testing.T) {
		rec := &recordingProvider{response: "ok"}
		cfg := &ExecutorConfig{
			Provider: rec,
			Results:  t.TempDir(),
			Logger:   slog.Default(),
		}
		job := &CronJob{Name: "n", Schedule: "0 9 * * *", Prompt: "p"}
		Execute(context.Background(), cfg, job)
		if rec.hadDeadline {
			t.Errorf("expected no deadline when Timeout is zero")
		}
	})

	t.Run("timeout set means provider sees deadline", func(t *testing.T) {
		rec := &recordingProvider{response: "ok"}
		cfg := &ExecutorConfig{
			Provider: rec,
			Results:  t.TempDir(),
			Logger:   slog.Default(),
		}
		job := &CronJob{Name: "n", Schedule: "0 9 * * *", Timeout: time.Second, Prompt: "p"}
		Execute(context.Background(), cfg, job)
		if !rec.hadDeadline {
			t.Errorf("expected provider ctx to carry a deadline")
		}
	})
}
