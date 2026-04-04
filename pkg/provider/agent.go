package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"wildgecu/x/debug"
)

// ErrDone is a sentinel error returned by a ToolExecutor to signal
// that the agent loop should terminate early (e.g. bootstrap's write_soul).
var ErrDone = errors.New("agent loop done")

// ToolExecutor is called for each tool call the model makes.
// It returns the tool result string. Return ErrDone to stop the loop early.
type ToolExecutor func(ctx context.Context, tc ToolCall) (string, error)

// ToolCallCallback is invoked before each tool execution with the full ToolCall.
type ToolCallCallback func(tc ToolCall)

// RunAgentLoopStream is like RunAgentLoop but streams the final text response.
func RunAgentLoopStream(ctx context.Context, p Provider, systemPrompt string, messages []Message, tools []Tool, execute ToolExecutor, onChunk StreamCallback, onToolCall ToolCallCallback, dbg *debug.Logger) ([]Message, *Response, error) {
	sp, canStream := p.(StreamProvider)
	if !canStream {
		return RunAgentLoop(ctx, p, systemPrompt, messages, tools, execute, onToolCall, dbg)
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

		dbg.ModelResponse(fullContent)
		messages = append(messages, resp.Message)

		toolMessages, err := executeToolsParallel(ctx, resp.Message.ToolCalls, execute, onToolCall, dbg)
		messages = append(messages, toolMessages...)
		if err != nil {
			return messages, resp, err
		}
	}
}

// RunAgentLoop runs the generate-execute cycle until the model produces
// a text response (no tool calls) or the executor signals ErrDone.
func RunAgentLoop(ctx context.Context, p Provider, systemPrompt string, messages []Message, tools []Tool, execute ToolExecutor, onToolCall ToolCallCallback, dbg *debug.Logger) ([]Message, *Response, error) {
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

		toolMessages, err := executeToolsParallel(ctx, resp.Message.ToolCalls, execute, onToolCall, dbg)
		messages = append(messages, toolMessages...)
		if err != nil {
			return messages, resp, err
		}
	}
}

// executeToolsParallel runs multiple tool calls concurrently and collects results.
func executeToolsParallel(ctx context.Context, toolCalls []ToolCall, execute ToolExecutor, onToolCall ToolCallCallback, dbg *debug.Logger) ([]Message, error) {
	var wg sync.WaitGroup
	msgs := make([]Message, len(toolCalls))
	errs := make([]error, len(toolCalls))

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(i int, tc ToolCall) {
			defer wg.Done()
			msgs[i], errs[i] = executeOne(ctx, tc, execute, onToolCall, dbg)
		}(i, tc)
	}
	wg.Wait()

	// Find if any tool returned the sentinel ErrDone
	var firstErr error
	for _, err := range errs {
		if errors.Is(err, ErrDone) {
			firstErr = ErrDone
			break
		}
	}

	return msgs, firstErr
}

// executeOne handles the execution and logging of a single tool call.
func executeOne(ctx context.Context, tc ToolCall, execute ToolExecutor, onToolCall ToolCallCallback, dbg *debug.Logger) (Message, error) {
	dbg.ToolCall(tc.Name, tc.Args)
	if onToolCall != nil {
		onToolCall(tc)
	}

	result, err := execute(ctx, tc)

	// If it's a real error (not the sentinel), format it for the LLM
	if err != nil && !errors.Is(err, ErrDone) {
		dbg.Error(err)
		result = fmt.Sprintf("Error: %v", err)
	} else {
		dbg.ToolResult(tc.Name, result)
	}

	return Message{
		Role:       RoleTool,
		Content:    result,
		ToolCallID: tc.Name,
	}, err
}
