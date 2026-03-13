package provider

import (
	"context"
	"errors"

	"gonesis/debug"
)

// ErrDone is a sentinel error returned by a ToolExecutor to signal
// that the agent loop should terminate early (e.g. bootstrap's write_soul).
var ErrDone = errors.New("agent loop done")

// ToolExecutor is called for each tool call the model makes.
// It returns the tool result string. Return ErrDone to stop the loop early.
type ToolExecutor func(ctx context.Context, tc ToolCall) (string, error)

// RunAgentLoopStream is like RunAgentLoop but streams the final text response.
// Intermediate tool-call iterations use blocking Generate. Only the final
// text-only response streams chunks via onChunk.
func RunAgentLoopStream(ctx context.Context, p Provider, systemPrompt string, messages []Message, tools []Tool, execute ToolExecutor, onChunk StreamCallback, dbg *debug.Logger) ([]Message, *Response, error) {
	sp, canStream := p.(StreamProvider)
	if !canStream {
		return RunAgentLoop(ctx, p, systemPrompt, messages, tools, execute, dbg)
	}

	for {
		dbg.GenerateRequest(len(messages), len(tools))

		chunks, errCh := sp.GenerateStream(ctx, &GenerateParams{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        tools,
		})

		var fullContent string
		var lastUsage Usage
		var toolCalls []ToolCall
		for chunk := range chunks {
			fullContent += chunk.Content
			if chunk.Usage.InputTokens > 0 || chunk.Usage.OutputTokens > 0 {
				lastUsage = chunk.Usage
			}
			toolCalls = append(toolCalls, chunk.ToolCalls...)
			if chunk.Content != "" {
				onChunk(chunk.Content)
			}
		}
		if err := <-errCh; err != nil {
			dbg.Error(err)
			return messages, nil, err
		}

		resp := &Response{
			Message: Message{Role: RoleModel, Content: fullContent, ToolCalls: toolCalls},
			Usage:   lastUsage,
		}

		dbg.Usage(lastUsage.InputTokens, lastUsage.OutputTokens)

		if len(resp.Message.ToolCalls) == 0 {
			dbg.ModelResponse(fullContent)
			messages = append(messages, resp.Message)
			return messages, resp, nil
		}

		// Tool calls detected — execute them, then loop with blocking Generate.
		dbg.ModelResponse(fullContent)
		messages = append(messages, resp.Message)
		for _, tc := range resp.Message.ToolCalls {
			dbg.ToolCall(tc.Name, tc.Args)
			result, err := execute(ctx, tc)
			if err != nil {
				if errors.Is(err, ErrDone) {
					dbg.ToolResult(tc.Name, result)
					return messages, resp, ErrDone
				}
				dbg.Error(err)
				return messages, resp, err
			}
			dbg.ToolResult(tc.Name, result)
			messages = append(messages, Message{Role: RoleTool, Content: result, ToolCallID: tc.Name})
		}
		// After tool execution, loop again (will stream again for next response)
	}
}

// RunAgentLoop runs the generate-execute cycle until the model produces
// a text response (no tool calls) or the executor signals ErrDone.
func RunAgentLoop(ctx context.Context, p Provider, systemPrompt string, messages []Message, tools []Tool, execute ToolExecutor, dbg *debug.Logger) ([]Message, *Response, error) {
	for {
		dbg.GenerateRequest(len(messages), len(tools))

		resp, err := p.Generate(ctx, &GenerateParams{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        tools,
		})
		if err != nil {
			dbg.Error(err)
			return messages, nil, err
		}

		dbg.Usage(resp.Usage.InputTokens, resp.Usage.OutputTokens)

		if len(resp.Message.ToolCalls) == 0 {
			dbg.ModelResponse(resp.Message.Content)
			messages = append(messages, resp.Message)
			return messages, resp, nil
		}

		dbg.ModelResponse(resp.Message.Content)
		messages = append(messages, resp.Message)

		for _, tc := range resp.Message.ToolCalls {
			dbg.ToolCall(tc.Name, tc.Args)
			result, err := execute(ctx, tc)
			if err != nil {
				if errors.Is(err, ErrDone) {
					dbg.ToolResult(tc.Name, result)
					return messages, resp, ErrDone
				}
				dbg.Error(err)
				return messages, resp, err
			}
			dbg.ToolResult(tc.Name, result)
			messages = append(messages, Message{
				Role:       RoleTool,
				Content:    result,
				ToolCallID: tc.Name,
			})
		}
	}
}
