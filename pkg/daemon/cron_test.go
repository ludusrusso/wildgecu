package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ludusrusso/wildgecu/pkg/cron"
)

type stubReloader struct {
	calls int
	err   error
}

func (s *stubReloader) Reload(_ context.Context) error {
	s.calls++
	return s.err
}

func writeJob(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, cron.Filename(name))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write job: %v", err)
	}
	return path
}

func TestCronSuspendHandler(t *testing.T) {
	t.Run("suspends a running job and triggers reload", func(t *testing.T) {
		dir := t.TempDir()
		path := writeJob(t, dir, "foo", "---\nname: foo\ncron: \"0 9 * * *\"\n---\nprompt\n")
		reloader := &stubReloader{}
		h := cronSuspendHandler(context.Background(), dir, reloader)

		resp, err := h(&Request{Args: map[string]any{"name": "foo"}})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if !resp.OK {
			t.Fatalf("expected OK, got error: %s", resp.Error)
		}
		if reloader.calls != 1 {
			t.Errorf("expected 1 reload, got %d", reloader.calls)
		}
		data, _ := os.ReadFile(path)
		job, err := cron.Parse(data)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if !job.Suspended {
			t.Errorf("expected file to be suspended after handler")
		}
	})

	t.Run("no-op when already suspended", func(t *testing.T) {
		dir := t.TempDir()
		writeJob(t, dir, "foo", "---\nname: foo\ncron: \"0 9 * * *\"\nsuspended: true\n---\nprompt\n")
		reloader := &stubReloader{}
		h := cronSuspendHandler(context.Background(), dir, reloader)

		resp, err := h(&Request{Args: map[string]any{"name": "foo"}})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if !resp.OK {
			t.Fatalf("expected OK no-op, got error: %s", resp.Error)
		}
		msg, _ := resp.Payload.(string)
		if !strings.Contains(msg, "already suspended") {
			t.Errorf("expected 'already suspended' message, got %q", msg)
		}
		if reloader.calls != 0 {
			t.Errorf("expected no reload on no-op, got %d", reloader.calls)
		}
	})

	t.Run("unknown name returns error", func(t *testing.T) {
		dir := t.TempDir()
		reloader := &stubReloader{}
		h := cronSuspendHandler(context.Background(), dir, reloader)

		resp, _ := h(&Request{Args: map[string]any{"name": "ghost"}})
		if resp.OK {
			t.Fatalf("expected error for unknown name")
		}
		if !strings.Contains(resp.Error, "no job named") {
			t.Errorf("expected 'no job named' error, got %q", resp.Error)
		}
	})

	t.Run("broken config refuses", func(t *testing.T) {
		dir := t.TempDir()
		writeJob(t, dir, "broken", "---\nname: broken\n---\nno schedule\n")
		reloader := &stubReloader{}
		h := cronSuspendHandler(context.Background(), dir, reloader)

		resp, _ := h(&Request{Args: map[string]any{"name": "broken"}})
		if resp.OK {
			t.Fatalf("expected error for broken config")
		}
		if !strings.Contains(resp.Error, "config errors") {
			t.Errorf("expected 'config errors' message, got %q", resp.Error)
		}
	})

	t.Run("missing name argument", func(t *testing.T) {
		dir := t.TempDir()
		h := cronSuspendHandler(context.Background(), dir, &stubReloader{})

		resp, _ := h(&Request{Args: map[string]any{}})
		if resp.OK {
			t.Fatal("expected error for missing name")
		}
	})

	t.Run("reload failure propagates", func(t *testing.T) {
		dir := t.TempDir()
		writeJob(t, dir, "foo", "---\nname: foo\ncron: \"0 9 * * *\"\n---\nprompt\n")
		reloader := &stubReloader{err: errors.New("boom")}
		h := cronSuspendHandler(context.Background(), dir, reloader)

		resp, _ := h(&Request{Args: map[string]any{"name": "foo"}})
		if resp.OK {
			t.Fatal("expected reload error to surface")
		}
		if !strings.Contains(resp.Error, "boom") {
			t.Errorf("expected error to include reason, got %q", resp.Error)
		}
	})
}

type stubListSource struct {
	jobs []cron.JobInfo
}

func (s *stubListSource) ListJobs() []cron.JobInfo {
	return s.jobs
}

func TestCronListHandler(t *testing.T) {
	t.Run("returns scheduler state verbatim", func(t *testing.T) {
		source := &stubListSource{jobs: []cron.JobInfo{
			{Name: "a", Schedule: "0 9 * * *", Status: cron.StatusRunning, NextRun: "2026-04-20T09:00:00Z"},
			{Name: "b", Schedule: "0 9 * * *", Status: cron.StatusSuspended},
			{Name: "c", Status: cron.StatusError, Error: "missing schedule"},
		}}

		h := cronListHandler(source)
		resp, err := h(&Request{})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if !resp.OK {
			t.Fatalf("expected OK, got error: %s", resp.Error)
		}
		got, ok := resp.Payload.([]cron.JobInfo)
		if !ok {
			t.Fatalf("payload type = %T, want []cron.JobInfo", resp.Payload)
		}
		if len(got) != 3 {
			t.Fatalf("got %d jobs, want 3", len(got))
		}
		if got[0].Status != cron.StatusRunning || got[1].Status != cron.StatusSuspended || got[2].Status != cron.StatusError {
			t.Errorf("statuses not preserved: %+v", got)
		}
	})
}

func TestCronResumeHandler(t *testing.T) {
	t.Run("resumes a suspended job and triggers reload", func(t *testing.T) {
		dir := t.TempDir()
		path := writeJob(t, dir, "foo", "---\nname: foo\ncron: \"0 9 * * *\"\nsuspended: true\n---\nprompt\n")
		reloader := &stubReloader{}
		h := cronResumeHandler(context.Background(), dir, reloader)

		resp, err := h(&Request{Args: map[string]any{"name": "foo"}})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if !resp.OK {
			t.Fatalf("expected OK, got error: %s", resp.Error)
		}
		if reloader.calls != 1 {
			t.Errorf("expected 1 reload, got %d", reloader.calls)
		}
		data, _ := os.ReadFile(path)
		job, err := cron.Parse(data)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if job.Suspended {
			t.Errorf("expected file to be resumed after handler")
		}
	})

	t.Run("no-op when already running", func(t *testing.T) {
		dir := t.TempDir()
		writeJob(t, dir, "foo", "---\nname: foo\ncron: \"0 9 * * *\"\n---\nprompt\n")
		reloader := &stubReloader{}
		h := cronResumeHandler(context.Background(), dir, reloader)

		resp, err := h(&Request{Args: map[string]any{"name": "foo"}})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if !resp.OK {
			t.Fatalf("expected OK no-op, got error: %s", resp.Error)
		}
		msg, _ := resp.Payload.(string)
		if !strings.Contains(msg, "already running") {
			t.Errorf("expected 'already running' message, got %q", msg)
		}
		if reloader.calls != 0 {
			t.Errorf("expected no reload on no-op, got %d", reloader.calls)
		}
	})

	t.Run("unknown name returns error", func(t *testing.T) {
		dir := t.TempDir()
		h := cronResumeHandler(context.Background(), dir, &stubReloader{})

		resp, _ := h(&Request{Args: map[string]any{"name": "ghost"}})
		if resp.OK {
			t.Fatal("expected error for unknown name")
		}
		if !strings.Contains(resp.Error, "no job named") {
			t.Errorf("expected 'no job named' error, got %q", resp.Error)
		}
	})

	t.Run("broken config refuses", func(t *testing.T) {
		dir := t.TempDir()
		writeJob(t, dir, "broken", "---\nname: broken\n---\n")
		reloader := &stubReloader{}
		h := cronResumeHandler(context.Background(), dir, reloader)

		resp, _ := h(&Request{Args: map[string]any{"name": "broken"}})
		if resp.OK {
			t.Fatal("expected error for broken config")
		}
		if !strings.Contains(resp.Error, "config errors") {
			t.Errorf("expected 'config errors' message, got %q", resp.Error)
		}
	})
}
