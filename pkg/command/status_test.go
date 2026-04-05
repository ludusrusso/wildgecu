package command

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestStatusCommand(t *testing.T) {
	t.Run("returns formatted session info", func(t *testing.T) {
		cmd := NewStatusCommand(func(_ context.Context, id string) (StatusInfo, error) {
			return StatusInfo{
				SessionID:    "sess-abc-123",
				MessageCount: 42,
				Provider:     "openai",
				Model:        "gpt-4o",
				Uptime:       3*time.Hour + 15*time.Minute,
			}, nil
		})

		ctx := WithSessionID(context.Background(), "sess-abc-123")
		result, err := cmd.Execute(ctx, "")
		if err != nil {
			t.Fatalf("Execute() error: %v", err)
		}

		for _, want := range []string{"sess-abc-123", "42", "openai", "gpt-4o", "3h15m"} {
			if !strings.Contains(result, want) {
				t.Errorf("expected result to contain %q, got:\n%s", want, result)
			}
		}
	})

	t.Run("returns error when no session ID in context", func(t *testing.T) {
		cmd := NewStatusCommand(func(_ context.Context, id string) (StatusInfo, error) {
			return StatusInfo{}, nil
		})

		_, err := cmd.Execute(context.Background(), "")
		if err == nil {
			t.Fatal("expected error when session ID is missing from context")
		}
	})

	t.Run("propagates status function error", func(t *testing.T) {
		cmd := NewStatusCommand(func(_ context.Context, id string) (StatusInfo, error) {
			return StatusInfo{}, fmt.Errorf("session not found")
		})

		ctx := WithSessionID(context.Background(), "sess-123")
		_, err := cmd.Execute(ctx, "")
		if err == nil {
			t.Fatal("expected error from status function")
		}
	})

	t.Run("name and description", func(t *testing.T) {
		cmd := NewStatusCommand(nil)
		if cmd.Name() != "status" {
			t.Errorf("expected name %q, got %q", "status", cmd.Name())
		}
		if cmd.Description() == "" {
			t.Error("expected non-empty description")
		}
	})
}
