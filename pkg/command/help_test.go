package command

import (
	"context"
	"strings"
	"testing"
)

func TestHelpCommand(t *testing.T) {
	t.Run("lists commands sorted", func(t *testing.T) {
		r := NewRegistry("")
		r.Register(&stubCommand{name: "zoo", desc: "Last alphabetically"})
		r.Register(&stubCommand{name: "alpha", desc: "First alphabetically"})

		help := NewHelpCommand(r)
		r.Register(help)

		out, err := help.Execute(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(out, "/alpha") {
			t.Error("expected output to contain /alpha")
		}
		if !strings.Contains(out, "/help") {
			t.Error("expected output to contain /help")
		}
		if !strings.Contains(out, "/zoo") {
			t.Error("expected output to contain /zoo")
		}

		// Verify sorted order: alpha before help before zoo
		alphaIdx := strings.Index(out, "/alpha")
		helpIdx := strings.Index(out, "/help")
		zooIdx := strings.Index(out, "/zoo")
		if alphaIdx >= helpIdx || helpIdx >= zooIdx {
			t.Errorf("expected sorted order, got alpha=%d help=%d zoo=%d", alphaIdx, helpIdx, zooIdx)
		}
	})

	t.Run("empty registry", func(t *testing.T) {
		r := NewRegistry("")
		help := NewHelpCommand(r)

		out, err := help.Execute(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "No commands available." {
			t.Errorf("expected empty message, got %q", out)
		}
	})

	t.Run("includes skills", func(t *testing.T) {
		dir := t.TempDir()
		writeTestSkill(t, dir, "---\nname: deploy\ndescription: Deploy the app\n---\ncontent")

		r := NewRegistry(dir)
		help := NewHelpCommand(r)
		r.Register(help)

		out, err := help.Execute(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "/deploy") {
			t.Error("expected output to contain /deploy")
		}
		if !strings.Contains(out, "/help") {
			t.Error("expected output to contain /help")
		}
	})
}
