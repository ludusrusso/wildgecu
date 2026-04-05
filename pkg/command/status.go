package command

import (
	"context"
	"fmt"
	"time"
)

// StatusInfo holds the session information returned by StatusFunc.
type StatusInfo struct {
	SessionID    string
	MessageCount int
	ToolCalls    int
	SkillsLoaded int
	Provider     string
	Model        string
	Uptime       time.Duration
}

// StatusFunc returns session information for the given session ID.
type StatusFunc func(ctx context.Context, id string) (StatusInfo, error)

// StatusCommand displays current session information.
type StatusCommand struct {
	statusFn StatusFunc
}

// NewStatusCommand creates a /status command that delegates to statusFn.
func NewStatusCommand(statusFn StatusFunc) *StatusCommand {
	return &StatusCommand{statusFn: statusFn}
}

func (c *StatusCommand) Name() string        { return "status" }
func (c *StatusCommand) Description() string { return "Show current session info" }

func (c *StatusCommand) Execute(ctx context.Context, _ string) (string, error) {
	sessionID := SessionIDFromContext(ctx)
	if sessionID == "" {
		return "", fmt.Errorf("no active session")
	}
	info, err := c.statusFn(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("get session status: %w", err)
	}
	return fmt.Sprintf(
		"Session:  %s\nMessages: %d\nTools:    %d\nSkills:   %d\nProvider: %s\nModel:    %s\nUptime:   %s",
		info.SessionID, info.MessageCount, info.ToolCalls, info.SkillsLoaded, info.Provider, info.Model, info.Uptime,
	), nil
}
