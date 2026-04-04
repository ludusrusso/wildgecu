package provider

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"wildgecu/x/debug"
)

func noopLogger() *debug.Logger {
	return &debug.Logger{}
}

func TestExecuteToolsParallel_RunsConcurrently(t *testing.T) {
	var running atomic.Int32
	var maxConcurrent atomic.Int32

	toolCalls := []ToolCall{
		{Name: "tool_a", Args: map[string]any{"x": 1}},
		{Name: "tool_b", Args: map[string]any{"x": 2}},
		{Name: "tool_c", Args: map[string]any{"x": 3}},
	}

	executor := func(ctx context.Context, tc ToolCall) (string, error) {
		cur := running.Add(1)
		// Track max concurrency
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		running.Add(-1)
		return "ok:" + tc.Name, nil
	}

	msgs, err := executeToolsParallel(context.Background(), toolCalls, executor, nil, noopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if maxConcurrent.Load() < 2 {
		t.Errorf("expected concurrent execution, max concurrency was %d", maxConcurrent.Load())
	}

	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	// Results must be in the same order as toolCalls
	for i, tc := range toolCalls {
		if msgs[i].Role != RoleTool {
			t.Errorf("msgs[%d].Role = %q, want %q", i, msgs[i].Role, RoleTool)
		}
		if msgs[i].Content != "ok:"+tc.Name {
			t.Errorf("msgs[%d].Content = %q, want %q", i, msgs[i].Content, "ok:"+tc.Name)
		}
		if msgs[i].ToolCallID != tc.Name {
			t.Errorf("msgs[%d].ToolCallID = %q, want %q", i, msgs[i].ToolCallID, tc.Name)
		}
	}
}

func TestExecuteToolsParallel_PreservesOrder(t *testing.T) {
	toolCalls := []ToolCall{
		{Name: "slow", Args: map[string]any{}},
		{Name: "fast", Args: map[string]any{}},
	}

	executor := func(ctx context.Context, tc ToolCall) (string, error) {
		if tc.Name == "slow" {
			time.Sleep(80 * time.Millisecond)
		}
		return tc.Name + "_result", nil
	}

	msgs, err := executeToolsParallel(context.Background(), toolCalls, executor, nil, noopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msgs[0].Content != "slow_result" {
		t.Errorf("msgs[0] should be slow_result, got %q", msgs[0].Content)
	}
	if msgs[1].Content != "fast_result" {
		t.Errorf("msgs[1] should be fast_result, got %q", msgs[1].Content)
	}
}

func TestExecuteToolsParallel_ErrDone(t *testing.T) {
	toolCalls := []ToolCall{
		{Name: "normal", Args: map[string]any{}},
		{Name: "done", Args: map[string]any{}},
	}

	executor := func(ctx context.Context, tc ToolCall) (string, error) {
		if tc.Name == "done" {
			return "finished", ErrDone
		}
		return "ok", nil
	}

	msgs, err := executeToolsParallel(context.Background(), toolCalls, executor, nil, noopLogger())
	if !errors.Is(err, ErrDone) {
		t.Fatalf("expected ErrDone, got %v", err)
	}

	// All messages should still be returned
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "ok" {
		t.Errorf("msgs[0].Content = %q, want %q", msgs[0].Content, "ok")
	}
	if msgs[1].Content != "finished" {
		t.Errorf("msgs[1].Content = %q, want %q", msgs[1].Content, "finished")
	}
}

func TestExecuteToolsParallel_ErrorFormatsMessage(t *testing.T) {
	toolCalls := []ToolCall{
		{Name: "failing", Args: map[string]any{}},
	}

	executor := func(ctx context.Context, tc ToolCall) (string, error) {
		return "", errors.New("something broke")
	}

	msgs, err := executeToolsParallel(context.Background(), toolCalls, executor, nil, noopLogger())
	if err != nil {
		t.Fatalf("non-sentinel errors should not propagate, got %v", err)
	}

	if msgs[0].Content != "Error: something broke" {
		t.Errorf("expected formatted error, got %q", msgs[0].Content)
	}
}

func TestExecuteToolsParallel_CallbackInvoked(t *testing.T) {
	toolCalls := []ToolCall{
		{Name: "a", Args: map[string]any{}},
		{Name: "b", Args: map[string]any{}},
	}

	var callbackCount atomic.Int32
	callback := func(tc ToolCall) {
		callbackCount.Add(1)
	}

	executor := func(ctx context.Context, tc ToolCall) (string, error) {
		return "ok", nil
	}

	_, err := executeToolsParallel(context.Background(), toolCalls, executor, callback, noopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callbackCount.Load() != 2 {
		t.Errorf("expected callback called 2 times, got %d", callbackCount.Load())
	}
}

func TestExecuteToolsParallel_EmptyToolCalls(t *testing.T) {
	executor := func(ctx context.Context, tc ToolCall) (string, error) {
		t.Fatal("executor should not be called")
		return "", nil
	}

	msgs, err := executeToolsParallel(context.Background(), nil, executor, nil, noopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestExecuteToolsParallel_SingleToolCall(t *testing.T) {
	toolCalls := []ToolCall{
		{Name: "only", Args: map[string]any{"key": "val"}},
	}

	executor := func(ctx context.Context, tc ToolCall) (string, error) {
		return "single_result", nil
	}

	msgs, err := executeToolsParallel(context.Background(), toolCalls, executor, nil, noopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "single_result" {
		t.Errorf("got %q, want %q", msgs[0].Content, "single_result")
	}
}

func TestExecuteToolsParallel_MultipleErrDone(t *testing.T) {
	toolCalls := []ToolCall{
		{Name: "done1", Args: map[string]any{}},
		{Name: "done2", Args: map[string]any{}},
		{Name: "normal", Args: map[string]any{}},
	}

	executor := func(ctx context.Context, tc ToolCall) (string, error) {
		if tc.Name == "normal" {
			return "ok", nil
		}
		return "done", ErrDone
	}

	msgs, err := executeToolsParallel(context.Background(), toolCalls, executor, nil, noopLogger())
	if !errors.Is(err, ErrDone) {
		t.Fatalf("expected ErrDone, got %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
}
