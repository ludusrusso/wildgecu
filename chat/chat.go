package chat

import (
	"context"

	"gonesis/debug"
	"gonesis/provider"
)

// Config holds everything needed to run a conversational loop.
type Config struct {
	Provider        provider.Provider
	Executor        provider.ToolExecutor
	OnDone          func(messages []provider.Message)
	OnToolCall      provider.ToolCallCallback
	Debug           *debug.Logger
	SystemPrompt    string
	WelcomeText     string
	Tools           []provider.Tool
	InitialMessages []provider.Message
}

// RunTurn appends a user message to the conversation and runs one agent loop.
// It returns the updated messages and the model's response.
func RunTurn(ctx context.Context, cfg *Config, messages []provider.Message, userInput string) ([]provider.Message, *provider.Response, error) {
	cfg.Debug.UserMessage(userInput)
	messages = append(messages, provider.Message{
		Role:    provider.RoleUser,
		Content: userInput,
	})
	return provider.RunAgentLoop(ctx, cfg.Provider, cfg.SystemPrompt, messages, cfg.Tools, cfg.Executor, cfg.OnToolCall, cfg.Debug)
}

// RunInitialTurn runs the agent loop on pre-seeded messages without adding a user message.
func RunInitialTurn(ctx context.Context, cfg *Config, messages []provider.Message) ([]provider.Message, *provider.Response, error) {
	return provider.RunAgentLoop(ctx, cfg.Provider, cfg.SystemPrompt, messages, cfg.Tools, cfg.Executor, cfg.OnToolCall, cfg.Debug)
}

// RunTurnStream is like RunTurn but streams text chunks via onChunk.
func RunTurnStream(ctx context.Context, cfg *Config, messages []provider.Message, userInput string, onChunk provider.StreamCallback) ([]provider.Message, *provider.Response, error) {
	cfg.Debug.UserMessage(userInput)
	messages = append(messages, provider.Message{
		Role:    provider.RoleUser,
		Content: userInput,
	})
	return provider.RunAgentLoopStream(ctx, cfg.Provider, cfg.SystemPrompt, messages, cfg.Tools, cfg.Executor, onChunk, cfg.OnToolCall, cfg.Debug)
}

// RunInitialTurnStream is like RunInitialTurn but streams text chunks via onChunk.
func RunInitialTurnStream(ctx context.Context, cfg *Config, messages []provider.Message, onChunk provider.StreamCallback) ([]provider.Message, *provider.Response, error) {
	return provider.RunAgentLoopStream(ctx, cfg.Provider, cfg.SystemPrompt, messages, cfg.Tools, cfg.Executor, onChunk, cfg.OnToolCall, cfg.Debug)
}
