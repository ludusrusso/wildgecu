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

func TestSchedulerListJobs(t *testing.T) {
	t.Run("classifies running, suspended, and error jobs", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "active.md"), []byte("---\nname: active\ncron: \"0 9 * * *\"\n---\nprompt-active"), 0o644)
		os.WriteFile(filepath.Join(dir, "paused.md"), []byte("---\nname: paused\ncron: \"0 9 * * *\"\nsuspended: true\n---\nprompt-paused"), 0o644)
		os.WriteFile(filepath.Join(dir, "broken.md"), []byte("---\nname: broken\n---\nno schedule"), 0o644)

		s := newTestScheduler(t, dir)
		defer s.Stop()
		if err := s.LoadAndStart(context.Background()); err != nil {
			t.Fatalf("LoadAndStart: %v", err)
		}

		infos := s.ListJobs()
		if len(infos) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(infos))
		}

		byName := map[string]JobInfo{}
		for _, i := range infos {
			byName[i.Name] = i
		}
		if got := byName["active"].Status; got != StatusRunning {
			t.Errorf("active: expected running, got %q", got)
		}
		if got := byName["active"].NextRun; got == "" {
			t.Errorf("active: expected NextRun to be populated")
		}
		if got := byName["paused"].Status; got != StatusSuspended {
			t.Errorf("paused: expected suspended, got %q", got)
		}
		if got := byName["paused"].NextRun; got != "" {
			t.Errorf("paused: expected empty NextRun, got %q", got)
		}
		if got := byName["broken"].Status; got != StatusError {
			t.Errorf("broken: expected error, got %q", got)
		}
		if byName["broken"].Error == "" {
			t.Errorf("broken: expected non-empty error reason")
		}
	})

	t.Run("reload reclassifies running → suspended → error → running", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "toggle.md")
		os.WriteFile(path, []byte("---\nname: toggle\ncron: \"0 9 * * *\"\n---\nprompt"), 0o644)

		s := newTestScheduler(t, dir)
		defer s.Stop()
		if err := s.LoadAndStart(context.Background()); err != nil {
			t.Fatalf("LoadAndStart: %v", err)
		}
		if got := firstStatus(t, s.ListJobs()); got != StatusRunning {
			t.Fatalf("stage1: expected running, got %q", got)
		}

		os.WriteFile(path, []byte("---\nname: toggle\ncron: \"0 9 * * *\"\nsuspended: true\n---\nprompt"), 0o644)
		if err := s.Reload(context.Background()); err != nil {
			t.Fatalf("Reload: %v", err)
		}
		if got := firstStatus(t, s.ListJobs()); got != StatusSuspended {
			t.Fatalf("stage2: expected suspended, got %q", got)
		}

		os.WriteFile(path, []byte("---\nname: toggle\n---\nno schedule"), 0o644)
		if err := s.Reload(context.Background()); err != nil {
			t.Fatalf("Reload: %v", err)
		}
		if got := firstStatus(t, s.ListJobs()); got != StatusError {
			t.Fatalf("stage3: expected error, got %q", got)
		}

		os.WriteFile(path, []byte("---\nname: toggle\ncron: \"0 9 * * *\"\n---\nprompt"), 0o644)
		if err := s.Reload(context.Background()); err != nil {
			t.Fatalf("Reload: %v", err)
		}
		if got := firstStatus(t, s.ListJobs()); got != StatusRunning {
			t.Fatalf("stage4: expected running, got %q", got)
		}
	})

	t.Run("broken job does not block other jobs", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "ok.md"), []byte("---\nname: ok\ncron: \"0 9 * * *\"\n---\nprompt"), 0o644)
		os.WriteFile(filepath.Join(dir, "broken.md"), []byte("---\nname: broken\n---\nno schedule"), 0o644)

		s := newTestScheduler(t, dir)
		defer s.Stop()
		if err := s.LoadAndStart(context.Background()); err != nil {
			t.Fatalf("LoadAndStart: %v", err)
		}
		if got := s.JobCount(); got != 1 {
			t.Errorf("expected 1 running job, got %d", got)
		}
		if got := len(s.ListJobs()); got != 2 {
			t.Errorf("expected 2 total entries, got %d", got)
		}
	})
}

func firstStatus(t *testing.T, infos []JobInfo) JobStatus {
	t.Helper()
	if len(infos) != 1 {
		t.Fatalf("expected 1 job, got %d: %+v", len(infos), infos)
	}
	return infos[0].Status
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
