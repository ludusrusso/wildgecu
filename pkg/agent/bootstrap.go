package agent

import (
	"context"
	"fmt"

	"wildgecu/x/home"
	"wildgecu/pkg/provider"
	"wildgecu/pkg/provider/tool"
	"wildgecu/pkg/session"
)

// WriteSoulInput is the input for the write_soul tool.
type WriteSoulInput struct {
	Content string `json:"content" description:"The full Markdown content for SOUL.md"`
}

// WriteSoulOutput is the output for the write_soul tool.
type WriteSoulOutput struct {
	Status string `json:"status"`
}

// BootstrapConfig returns a session.Config for the bootstrap interview flow.
// The executor writes SOUL.md and signals ErrDone; soulContent is populated
// via the pointer so the caller can read it after tui.Run returns.
func BootstrapConfig(ctx context.Context, p provider.Provider, h home.Home, soulContent *string) *session.Config {
	writeSoulTool := tool.NewTool("write_soul",
		"Write your SOUL.md -- commit your identity to memory. Call this when you understand who you are.",
		func(ctx context.Context, in WriteSoulInput) (WriteSoulOutput, error) {
			if in.Content == "" {
				return WriteSoulOutput{}, fmt.Errorf("content must not be empty")
			}
			if err := writeSoul(h, in.Content); err != nil {
				return WriteSoulOutput{}, fmt.Errorf("bootstrap write: %w", err)
			}
			*soulContent = in.Content
			return WriteSoulOutput{Status: "ok"}, provider.ErrDone
		},
	)

	registry := tool.NewRegistry(writeSoulTool)

	return &session.Config{
		Provider:     p,
		SystemPrompt: bootstrapPrompt,
		Tools:        registry.Tools(),
		Executor:     registry.Executor(),
		InitialMessages: []provider.Message{
			{Role: provider.RoleUser, Content: "Hey! Let's set you up."},
		},
		WelcomeText: "Setting up a new agent...",
	}
}
