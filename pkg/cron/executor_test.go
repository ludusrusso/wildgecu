package cron

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wildgecu/pkg/provider"
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
