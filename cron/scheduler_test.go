package cron

import (
	"context"
	"log/slog"
	"testing"

	"wildgecu/homer"
)

func newTestScheduler(t *testing.T, crons homer.Homer) *Scheduler {
	t.Helper()
	results := homer.NewMem()
	cfg := &ExecutorConfig{
		Provider: &mockProvider{response: "test"},
		Results:  results,
		Logger:   slog.Default(),
	}
	s, err := NewScheduler(crons, cfg, slog.Default())
	if err != nil {
		t.Fatalf("NewScheduler failed: %v", err)
	}
	return s
}

func TestSchedulerLoadAndStart(t *testing.T) {
	crons := homer.NewMem()
	crons.Upsert("job1.md", []byte("---\nname: job1\ncron: \"0 9 * * *\"\n---\nprompt1"))
	crons.Upsert("job2.md", []byte("---\nname: job2\ncron: \"0 18 * * *\"\n---\nprompt2"))

	s := newTestScheduler(t, crons)
	defer s.Stop()

	if err := s.LoadAndStart(context.Background()); err != nil {
		t.Fatalf("LoadAndStart failed: %v", err)
	}

	if got := s.JobCount(); got != 2 {
		t.Errorf("expected 2 jobs, got %d", got)
	}
}

func TestSchedulerReload(t *testing.T) {
	crons := homer.NewMem()
	crons.Upsert("job1.md", []byte("---\nname: job1\ncron: \"0 9 * * *\"\n---\nprompt1"))

	s := newTestScheduler(t, crons)
	defer s.Stop()

	if err := s.LoadAndStart(context.Background()); err != nil {
		t.Fatalf("LoadAndStart failed: %v", err)
	}

	if got := s.JobCount(); got != 1 {
		t.Errorf("expected 1 job, got %d", got)
	}

	// Add another job and reload
	crons.Upsert("job2.md", []byte("---\nname: job2\ncron: \"0 18 * * *\"\n---\nprompt2"))

	if err := s.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if got := s.JobCount(); got != 2 {
		t.Errorf("expected 2 jobs after reload, got %d", got)
	}
}

func TestSchedulerReloadRemovesDeletedJobs(t *testing.T) {
	crons := homer.NewMem()
	crons.Upsert("job1.md", []byte("---\nname: job1\ncron: \"0 9 * * *\"\n---\nprompt1"))
	crons.Upsert("job2.md", []byte("---\nname: job2\ncron: \"0 18 * * *\"\n---\nprompt2"))

	s := newTestScheduler(t, crons)
	defer s.Stop()

	if err := s.LoadAndStart(context.Background()); err != nil {
		t.Fatalf("LoadAndStart failed: %v", err)
	}

	if got := s.JobCount(); got != 2 {
		t.Errorf("expected 2 jobs, got %d", got)
	}

	// Remove a job and reload
	crons.Delete("job2.md")

	if err := s.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if got := s.JobCount(); got != 1 {
		t.Errorf("expected 1 job after reload, got %d", got)
	}
}

func TestSchedulerEmptyLoad(t *testing.T) {
	crons := homer.NewMem()
	s := newTestScheduler(t, crons)
	defer s.Stop()

	if err := s.LoadAndStart(context.Background()); err != nil {
		t.Fatalf("LoadAndStart failed: %v", err)
	}

	if got := s.JobCount(); got != 0 {
		t.Errorf("expected 0 jobs, got %d", got)
	}
}
