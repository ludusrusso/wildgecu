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

func TestSchedulerSuspended(t *testing.T) {
	t.Run("does not register suspended jobs on LoadAndStart", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "active.md"), []byte("---\nname: active\ncron: \"0 9 * * *\"\n---\nprompt"), 0o644)
		os.WriteFile(filepath.Join(dir, "paused.md"), []byte("---\nname: paused\ncron: \"0 9 * * *\"\nsuspended: true\n---\nprompt"), 0o644)

		s := newTestScheduler(t, dir)
		defer s.Stop()

		if err := s.LoadAndStart(context.Background()); err != nil {
			t.Fatalf("LoadAndStart failed: %v", err)
		}
		if got := s.JobCount(); got != 1 {
			t.Errorf("expected 1 registered job (suspended skipped), got %d", got)
		}
	})

	t.Run("reload picks up a job when suspended flag flips to false", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "toggle.md")
		os.WriteFile(path, []byte("---\nname: toggle\ncron: \"0 9 * * *\"\nsuspended: true\n---\nprompt"), 0o644)

		s := newTestScheduler(t, dir)
		defer s.Stop()

		if err := s.LoadAndStart(context.Background()); err != nil {
			t.Fatalf("LoadAndStart failed: %v", err)
		}
		if got := s.JobCount(); got != 0 {
			t.Errorf("expected 0 registered jobs while suspended, got %d", got)
		}

		os.WriteFile(path, []byte("---\nname: toggle\ncron: \"0 9 * * *\"\n---\nprompt"), 0o644)

		if err := s.Reload(context.Background()); err != nil {
			t.Fatalf("Reload failed: %v", err)
		}
		if got := s.JobCount(); got != 1 {
			t.Errorf("expected 1 registered job after resume, got %d", got)
		}
	})

	t.Run("reload drops a job when suspended flag flips to true", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "toggle.md")
		os.WriteFile(path, []byte("---\nname: toggle\ncron: \"0 9 * * *\"\n---\nprompt"), 0o644)

		s := newTestScheduler(t, dir)
		defer s.Stop()

		if err := s.LoadAndStart(context.Background()); err != nil {
			t.Fatalf("LoadAndStart failed: %v", err)
		}
		if got := s.JobCount(); got != 1 {
			t.Errorf("expected 1 registered job, got %d", got)
		}

		os.WriteFile(path, []byte("---\nname: toggle\ncron: \"0 9 * * *\"\nsuspended: true\n---\nprompt"), 0o644)

		if err := s.Reload(context.Background()); err != nil {
			t.Fatalf("Reload failed: %v", err)
		}
		if got := s.JobCount(); got != 0 {
			t.Errorf("expected 0 registered jobs after suspend, got %d", got)
		}
	})
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
