package cron

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"wildgecu/x/home"
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
	results := home.NewMem()
	cfg := &ExecutorConfig{
		Provider: &mockProvider{response: "Here is your summary"},
		Results:  results,
		Logger:   slog.Default(),
	}

	job := &CronJob{
		Name:     "daily-summary",
		Schedule: "0 9 * * *",
		Prompt:   "Summarize my day",
	}

	Execute(context.Background(), cfg, job)

	files, err := results.Search("daily-summary-*.md")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 result file, got %d", len(files))
	}

	data, err := results.Get(files[0])
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(data) != "Here is your summary" {
		t.Errorf("expected result content, got %q", data)
	}

	if !strings.HasPrefix(files[0], "daily-summary-") {
		t.Errorf("expected filename to start with daily-summary-, got %q", files[0])
	}
	if !strings.HasSuffix(files[0], ".md") {
		t.Errorf("expected filename to end with .md, got %q", files[0])
	}
}
