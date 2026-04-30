package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ludusrusso/wildgecu/x/debug"
)

// ErrDone is a sentinel returned by a ToolExecutor to signal that the tool
// produced the terminal result the caller was waiting for and the agent
// loop should stop — it is not an error condition. Callers distinguish it
// from real failures with errors.Is(err, ErrDone).
//
// Used by one-shot "finalizer" tools such as write_memory and write_soul.
var ErrDone = errors.New("agent loop done")

// ToolExecutor is called for each tool call the model makes.
// It returns the tool result string. Return ErrDone to stop the loop early.
type ToolExecutor func(ctx context.Context, tc ToolCall) (string, error)

// RunAgentLoopStream is like RunAgentLoop but streams the final text response.
func RunAgentLoopStream(ctx context.Context, p Provider, systemPrompt string, messages []Message, toolset ToolSet, onChunk StreamCallback, onToolCall ToolCallCallback, dbg *debug.Logger) ([]Message, *Response, error) {
	if onToolCall != nil {
		ctx = WithToolCallCallback(ctx, onToolCall)
	}

	tools, execute := unpackToolSet(toolset)

	sp, canStream := p.(StreamProvider)
	if !canStream {
		return RunAgentLoop(ctx, p, systemPrompt, messages, toolset, onToolCall, dbg)
	}

	for {
		dbg.GenerateRequest(len(messages), len(tools))

		chunks, errCh := sp.GenerateStream(ctx, &GenerateParams{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        tools,
		})

		var fullContent strings.Builder
		var lastUsage Usage
		var toolCalls []ToolCall
		for chunk := range chunks {
			fullContent.WriteString(chunk.Content)
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

		contentStr := fullContent.String()

		resp := &Response{
			Message: Message{Role: RoleModel, Content: contentStr, ToolCalls: toolCalls},
			Usage:   lastUsage,
		}

		dbg.Usage(lastUsage.InputTokens, lastUsage.OutputTokens)

		if len(resp.Message.ToolCalls) == 0 {
			dbg.ModelResponse(contentStr)
			messages = append(messages, resp.Message)
			return messages, resp, nil
		}

		dbg.ModelResponse(contentStr)
		messages = append(messages, resp.Message)

		toolMessages, done := executeToolsParallel(ctx, resp.Message.ToolCalls, execute, onToolCall, dbg)
		messages = append(messages, toolMessages...)
		if done {
			return messages, resp, ErrDone
		}
	}
}

// RunAgentLoop runs the generate-execute cycle until the model produces
// a text response (no tool calls) or the executor signals ErrDone.
func RunAgentLoop(ctx context.Context, p Provider, systemPrompt string, messages []Message, toolset ToolSet, onToolCall ToolCallCallback, dbg *debug.Logger) ([]Message, *Response, error) {
	if onToolCall != nil {
		ctx = WithToolCallCallback(ctx, onToolCall)
	}

	tools, execute := unpackToolSet(toolset)

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

		toolMessages, done := executeToolsParallel(ctx, resp.Message.ToolCalls, execute, onToolCall, dbg)
		messages = append(messages, toolMessages...)
		if done {
			return messages, resp, ErrDone
		}
	}
}

// executeToolsParallel runs multiple tool calls concurrently and collects
// results. The bool is true if any tool signalled ErrDone.
func executeToolsParallel(ctx context.Context, toolCalls []ToolCall, execute ToolExecutor, onToolCall ToolCallCallback, dbg *debug.Logger) ([]Message, bool) {
	var wg sync.WaitGroup
	var done atomic.Bool
	msgs := make([]Message, len(toolCalls))

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(i int, tc ToolCall) {
			defer wg.Done()
			msg, d := executeOne(ctx, tc, execute, onToolCall, dbg)
			msgs[i] = msg
			if d {
				done.Store(true)
			}
		}(i, tc)
	}
	wg.Wait()

	return msgs, done.Load()
}

// executeOne handles the execution and logging of a single tool call. Real
// tool errors are rendered into the returned Message so the model can see
// them; the bool is true iff the executor returned ErrDone.
func executeOne(ctx context.Context, tc ToolCall, execute ToolExecutor, onToolCall ToolCallCallback, dbg *debug.Logger) (Message, bool) {
	dbg.ToolCall(tc.Name, tc.Args)
	if onToolCall != nil {
		onToolCall(tc.Name, FormatToolArgs(tc.Args, 100), "")
	}

	result, err := execute(ctx, tc)
	done := errors.Is(err, ErrDone)

	if err != nil && !done {
		dbg.Error(err)
		result = fmt.Sprintf("Error: %v", err)
	} else {
		dbg.ToolResult(tc.Name, result)
	}

	return Message{
		Role:       RoleTool,
		Content:    result,
		ToolCallID: tc.ID,
	}, done
}

// FormatToolArgs formats a tool call's args map into a compact string.
func FormatToolArgs(args map[string]any, maxLen int) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s: %v", k, v))
	}
	result := strings.Join(parts, ", ")
	if len(result) > maxLen {
		result = result[:maxLen] + "..."
	}
	return result
}
