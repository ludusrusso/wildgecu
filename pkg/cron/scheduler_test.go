package cron

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func newTestScheduler(t *testing.T, cronsDir string) *Scheduler {
	t.Helper()
	resultsDir := t.TempDir()
	cfg := &ExecutorConfig{
		Provider: &mockProvider{response: "test"},
		Results:  resultsDir,
		Logger:   slog.Default(),
	}
	s, err := NewScheduler(cronsDir, cfg, slog.Default())
	if err != nil {
		t.Fatalf("NewScheduler failed: %v", err)
	}
	return s
}

func TestSchedulerLoadAndStart(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "job1.md"), []byte("---\nname: job1\ncron: \"0 9 * * *\"\n---\nprompt1"), 0o644)
	os.WriteFile(filepath.Join(dir, "job2.md"), []byte("---\nname: job2\ncron: \"0 18 * * *\"\n---\nprompt2"), 0o644)

	s := newTestScheduler(t, dir)
	defer s.Stop()

	if err := s.LoadAndStart(context.Background()); err != nil {
		t.Fatalf("LoadAndStart failed: %v", err)
	}

	if got := s.JobCount(); got != 2 {
		t.Errorf("expected 2 jobs, got %d", got)
	}
}

func TestSchedulerReload(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "job1.md"), []byte("---\nname: job1\ncron: \"0 9 * * *\"\n---\nprompt1"), 0o644)

	s := newTestScheduler(t, dir)
	defer s.Stop()

	if err := s.LoadAndStart(context.Background()); err != nil {
		t.Fatalf("LoadAndStart failed: %v", err)
	}

	if got := s.JobCount(); got != 1 {
		t.Errorf("expected 1 job, got %d", got)
	}

	// Add another job and reload
	os.WriteFile(filepath.Join(dir, "job2.md"), []byte("---\nname: job2\ncron: \"0 18 * * *\"\n---\nprompt2"), 0o644)

	if err := s.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if got := s.JobCount(); got != 2 {
		t.Errorf("expected 2 jobs after reload, got %d", got)
	}
}

func TestSchedulerReloadRemovesDeletedJobs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "job1.md"), []byte("---\nname: job1\ncron: \"0 9 * * *\"\n---\nprompt1"), 0o644)
	os.WriteFile(filepath.Join(dir, "job2.md"), []byte("---\nname: job2\ncron: \"0 18 * * *\"\n---\nprompt2"), 0o644)

	s := newTestScheduler(t, dir)
	defer s.Stop()

	if err := s.LoadAndStart(context.Background()); err != nil {
		t.Fatalf("LoadAndStart failed: %v", err)
	}

	if got := s.JobCount(); got != 2 {
		t.Errorf("expected 2 jobs, got %d", got)
	}

	// Remove a job and reload
	os.Remove(filepath.Join(dir, "job2.md"))

	if err := s.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if got := s.JobCount(); got != 1 {
		t.Errorf("expected 1 job after reload, got %d", got)
	}
}

func TestSchedulerEmptyLoad(t *testing.T) {
	dir := t.TempDir()
	s := newTestScheduler(t, dir)
	defer s.Stop()

	if err := s.LoadAndStart(context.Background()); err != nil {
		t.Fatalf("LoadAndStart failed: %v", err)
	}

	if got := s.JobCount(); got != 0 {
		t.Errorf("expected 0 jobs, got %d", got)
	}
}
