package provider

import (
	"context"
	"testing"
)

func TestToolCallCallbackContext(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		var gotName, gotArgs, gotAgent string
		fn := ToolCallCallback(func(name, args, agent string) {
			gotName = name
			gotArgs = args
			gotAgent = agent
		})
		ctx := WithToolCallCallback(context.Background(), fn)
		got := GetToolCallCallback(ctx)
		if got == nil {
			t.Fatal("expected non-nil ToolCallCallback from context")
		}
		got("my_tool", "x: 1", "subagent")
		if gotName != "my_tool" {
			t.Errorf("name = %q, want %q", gotName, "my_tool")
		}
		if gotArgs != "x: 1" {
			t.Errorf("args = %q, want %q", gotArgs, "x: 1")
		}
		if gotAgent != "subagent" {
			t.Errorf("agent = %q, want %q", gotAgent, "subagent")
		}
	})

	t.Run("nil when not set", func(t *testing.T) {
		got := GetToolCallCallback(context.Background())
		if got != nil {
			t.Error("expected nil ToolCallCallback from bare context")
		}
	})
}
