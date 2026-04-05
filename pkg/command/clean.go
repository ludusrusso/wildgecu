package command

import (
	"context"
	"fmt"
)

type sessionIDKey struct{}

// WithSessionID returns a context carrying the given session ID.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, id)
}

// SessionIDFromContext extracts the session ID from ctx, or returns "".
func SessionIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(sessionIDKey{}).(string)
	return id
}

// ResetFunc resets the session identified by id and returns the new session ID.
type ResetFunc func(ctx context.Context, id string) (string, error)

// CleanCommand resets the current session.
type CleanCommand struct {
	resetFn ResetFunc
}

// NewCleanCommand creates a /clean command that delegates to resetFn.
func NewCleanCommand(resetFn ResetFunc) *CleanCommand {
	return &CleanCommand{resetFn: resetFn}
}

func (c *CleanCommand) Name() string        { return "clean" }
func (c *CleanCommand) Description() string { return "Reset the current session" }

func (c *CleanCommand) Execute(ctx context.Context, _ string) (string, error) {
	sessionID := SessionIDFromContext(ctx)
	if sessionID == "" {
		return "", fmt.Errorf("no active session")
	}
	newID, err := c.resetFn(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("reset session: %w", err)
	}
	return fmt.Sprintf("Session reset. New session: %s", newID), nil
}
