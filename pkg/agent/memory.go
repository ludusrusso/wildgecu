package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ludusrusso/wildgecu/pkg/home"
	"github.com/ludusrusso/wildgecu/pkg/provider"
	"github.com/ludusrusso/wildgecu/pkg/provider/tool"
)

// WriteMemoryInput is the input for the write_memory tool.
type WriteMemoryInput struct {
	Content string `json:"content" description:"The full Markdown content for MEMORY.md"`
}

// WriteMemoryOutput is the output for the write_memory tool.
type WriteMemoryOutput struct {
	Status string `json:"status"`
}

// RunMemoryAgent reviews the conversation and updates MEMORY.md.
func RunMemoryAgent(ctx context.Context, p provider.Provider, h *home.Home, messages []provider.Message, currentMemory string) error {
	start := time.Now()
	slog.Info("memory agent: start", "messages", len(messages), "memory_bytes", len(currentMemory))
	defer func() {
		slog.Info("memory agent: done", "elapsed", time.Since(start))
	}()

	writeMemoryTool := tool.NewTool("write_memory",
		"Write the updated MEMORY.md content.",
		func(ctx context.Context, in WriteMemoryInput) (WriteMemoryOutput, error) {
			if in.Content == "" {
				return WriteMemoryOutput{}, fmt.Errorf("content must not be empty")
			}
			if err := h.Memory().Write(in.Content); err != nil {
				return WriteMemoryOutput{}, fmt.Errorf("writing MEMORY.md: %w", err)
			}
			slog.Info("memory agent: write_memory invoked", "bytes", len(in.Content))
			return WriteMemoryOutput{Status: "ok"}, provider.ErrDone
		},
	)

	registry := tool.NewRegistry(writeMemoryTool)

	transcript := formatTranscript(messages)

	var userMsg strings.Builder
	userMsg.WriteString("## Conversation Transcript\n\n")
	userMsg.WriteString(transcript)
	userMsg.WriteString("\n\n## Current MEMORY.md\n\n")
	if currentMemory == "" {
		userMsg.WriteString("(empty — no existing memory)")
	} else {
		userMsg.WriteString(currentMemory)
	}
	userMsg.WriteString("\n\nReview the conversation above and call `write_memory` with the updated memory content.")

	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: userMsg.String()},
	}

	_, _, err := provider.RunAgentLoop(ctx, p, memoryAgentPrompt, msgs, registry.Tools(), registry.Executor(), nil, nil)
	if err != nil && !errors.Is(err, provider.ErrDone) {
		return fmt.Errorf("memory agent: %w", err)
	}
	return nil
}

// formatTranscript converts conversation messages into a readable transcript.
func formatTranscript(messages []provider.Message) string {
	var b strings.Builder
	for _, m := range messages {
		switch m.Role {
		case provider.RoleUser:
			fmt.Fprintf(&b, "**User:** %s\n\n", m.Content)
		case provider.RoleModel:
			if m.Content != "" {
				fmt.Fprintf(&b, "**Assistant:** %s\n\n", m.Content)
			}
			for _, tc := range m.ToolCalls {
				fmt.Fprintf(&b, "**Assistant** called tool `%s`\n\n", tc.Name)
			}
		case provider.RoleTool:
			// Keep tool results brief
			result := m.Content
			if len(result) > 200 {
				result = result[:200] + "..."
			}
			fmt.Fprintf(&b, "**Tool result:** %s\n\n", result)
		}
	}
	return b.String()
}
