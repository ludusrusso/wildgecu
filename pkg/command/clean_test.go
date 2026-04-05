package command

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestCleanCommand(t *testing.T) {
	t.Run("resets session and returns confirmation", func(t *testing.T) {
		resetCalled := false
		cmd := NewCleanCommand(func(ctx context.Context, id string) (string, error) {
			resetCalled = true
			return "new-session-456", nil
		})

		ctx := WithSessionID(context.Background(), "old-session-123")
		result, err := cmd.Execute(ctx, "")
		if err != nil {
			t.Fatalf("Execute() error: %v", err)
		}
		if !resetCalled {
			t.Error("expected reset function to be called")
		}
		if !strings.Contains(result, "new-session-456") {
			t.Errorf("expected result to contain new session ID, got %q", result)
		}
	})

	t.Run("returns error when no session ID in context", func(t *testing.T) {
		cmd := NewCleanCommand(func(ctx context.Context, id string) (string, error) {
			return "", nil
		})

		_, err := cmd.Execute(context.Background(), "")
		if err == nil {
			t.Fatal("expected error when session ID is missing from context")
		}
	})

	t.Run("propagates reset error", func(t *testing.T) {
		cmd := NewCleanCommand(func(ctx context.Context, id string) (string, error) {
			return "", fmt.Errorf("finalize failed")
		})

		ctx := WithSessionID(context.Background(), "session-123")
		_, err := cmd.Execute(ctx, "")
		if err == nil {
			t.Fatal("expected error from reset function")
		}
	})

	t.Run("name and description", func(t *testing.T) {
		cmd := NewCleanCommand(nil)
		if cmd.Name() != "clean" {
			t.Errorf("expected name %q, got %q", "clean", cmd.Name())
		}
		if cmd.Description() == "" {
			t.Error("expected non-empty description")
		}
	})
}

func TestSessionIDContext(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		ctx := WithSessionID(context.Background(), "abc-123")
		got := SessionIDFromContext(ctx)
		if got != "abc-123" {
			t.Errorf("expected %q, got %q", "abc-123", got)
		}
	})

	t.Run("empty when not set", func(t *testing.T) {
		got := SessionIDFromContext(context.Background())
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}
