package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ludusrusso/wildgecu/pkg/cron"
	"github.com/ludusrusso/wildgecu/pkg/daemon"
)

func TestRunCronLs(t *testing.T) {
	t.Run("daemon up renders STATUS/LAST RUN/NEXT RUN with mixed states", func(t *testing.T) {
		payload := []cron.JobInfo{
			{
				Name:     "active",
				Schedule: "0 9 * * *",
				Status:   cron.StatusRunning,
				Prompt:   "say hello",
				NextRun:  "2026-04-20T09:00:00Z",
				LastRun:  "2026-04-19T09:00:00Z",
			},
			{
				Name:     "paused",
				Schedule: "0 9 * * *",
				Status:   cron.StatusSuspended,
				Prompt:   "say goodbye",
			},
			{
				Name:   "broken",
				Status: cron.StatusError,
				Error:  "missing schedule",
			},
		}

		stub := func(cmd string, args map[string]any) (*daemon.Response, error) {
			if cmd != "cron-list" {
				t.Fatalf("unexpected command %q", cmd)
			}
			raw, _ := json.Marshal(payload)
			var generic any
			_ = json.Unmarshal(raw, &generic)
			return &daemon.Response{OK: true, Payload: generic}, nil
		}

		var stdout, stderr bytes.Buffer
		if err := runCronLs(&stdout, &stderr, t.TempDir(), true, stub); err != nil {
			t.Fatalf("runCronLs: %v", err)
		}

		out := stdout.String()
		for _, want := range []string{"NAME", "SCHEDULE", "STATUS", "LAST RUN", "NEXT RUN", "PROMPT",
			"active", "running", "paused", "suspended", "broken", "error: missing schedule"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("daemon down falls back to filesystem listing", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "good.md"), []byte("---\nname: good\ncron: \"0 9 * * *\"\n---\nprompt"), 0o644)
		os.WriteFile(filepath.Join(dir, "bad.md"), []byte("---\nname: bad\n---\nmissing schedule"), 0o644)

		stub := func(cmd string, args map[string]any) (*daemon.Response, error) {
			t.Fatal("should not call daemon in fallback mode")
			return nil, nil
		}

		var stdout, stderr bytes.Buffer
		if err := runCronLs(&stdout, &stderr, dir, false, stub); err != nil {
			t.Fatalf("runCronLs: %v", err)
		}

		out := stdout.String()
		if !strings.Contains(out, "(daemon offline)") {
			t.Errorf("expected '(daemon offline)' status, got:\n%s", out)
		}
		if strings.Contains(out, "LAST RUN") || strings.Contains(out, "NEXT RUN") {
			t.Errorf("fallback should omit LAST RUN/NEXT RUN:\n%s", out)
		}
		if !strings.Contains(out, "good") {
			t.Errorf("expected 'good' job in output:\n%s", out)
		}
		if !strings.Contains(out, "bad") {
			t.Errorf("expected 'bad' job (unparseable, by filename) in output:\n%s", out)
		}
	})

	t.Run("daemon reachable but errors falls through to filesystem", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "solo.md"), []byte("---\nname: solo\ncron: \"0 9 * * *\"\n---\nprompt"), 0o644)

		stub := func(cmd string, args map[string]any) (*daemon.Response, error) {
			return nil, errors.New("socket error")
		}

		var stdout, stderr bytes.Buffer
		if err := runCronLs(&stdout, &stderr, dir, true, stub); err != nil {
			t.Fatalf("runCronLs: %v", err)
		}
		if !strings.Contains(stderr.String(), "daemon query failed") {
			t.Errorf("expected warning about daemon failure, got stderr:\n%s", stderr.String())
		}
		if !strings.Contains(stdout.String(), "(daemon offline)") {
			t.Errorf("expected fallback rendering, got:\n%s", stdout.String())
		}
	})

	t.Run("empty directory prints helpful message", func(t *testing.T) {
		stub := func(cmd string, args map[string]any) (*daemon.Response, error) {
			return &daemon.Response{OK: true, Payload: []any{}}, nil
		}

		var stdout, stderr bytes.Buffer
		if err := runCronLs(&stdout, &stderr, t.TempDir(), true, stub); err != nil {
			t.Fatalf("runCronLs: %v", err)
		}
		if !strings.Contains(stdout.String(), "No cron jobs found") {
			t.Errorf("expected empty message, got:\n%s", stdout.String())
		}
	})
}
