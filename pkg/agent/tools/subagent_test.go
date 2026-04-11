package tools

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"testing"
	"time"

	"wildgecu/pkg/provider"
	"wildgecu/pkg/provider/tool"
)

// mockProvider records Generate calls and returns a canned response.
type mockProvider struct {
	calls    []provider.GenerateParams
	response *provider.Response
}

func (m *mockProvider) Generate(_ context.Context, params *provider.GenerateParams) (*provider.Response, error) {
	m.calls = append(m.calls, *params)
	return m.response, nil
}

func newMockProvider(content string) *mockProvider {
	return &mockProvider{
		response: &provider.Response{
			Message: provider.Message{Role: provider.RoleModel, Content: content},
		},
	}
}

// funcProvider delegates Generate to an arbitrary function.
type funcProvider struct {
	fn func(context.Context, *provider.GenerateParams) (*provider.Response, error)
}

func (p *funcProvider) Generate(ctx context.Context, params *provider.GenerateParams) (*provider.Response, error) {
	return p.fn(ctx, params)
}

func TestSubagentTools(t *testing.T) {
	t.Run("returns one tool named spawn_agent", func(t *testing.T) {
		tools := SubagentTools(nil, nil, nil)
		if len(tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(tools))
		}
		if got := tools[0].Definition().Name; got != "spawn_agent" {
			t.Errorf("expected tool name spawn_agent, got %q", got)
		}
	})

	t.Run("schema has prompt required and optional fields", func(t *testing.T) {
		tl := SubagentTools(nil, nil, nil)[0]
		def := tl.Definition()
		params := def.Parameters

		props, hasProps := params["properties"].(map[string]any)
		if !hasProps {
			t.Fatal("expected properties in schema")
		}
		if _, found := props["prompt"]; !found {
			t.Error("expected prompt in properties")
		}
		if _, found := props["system_prompt"]; !found {
			t.Error("expected system_prompt in properties")
		}
		if _, found := props["model"]; !found {
			t.Error("expected model in properties")
		}
		if _, found := props["tools"]; !found {
			t.Error("expected tools in properties")
		}
		if _, found := props["name"]; !found {
			t.Error("expected name in properties")
		}

		required, hasReq := params["required"].([]any)
		if !hasReq {
			t.Fatal("expected required in schema")
		}
		if len(required) != 1 || required[0] != "prompt" {
			t.Errorf("expected required=[prompt], got %v", required)
		}
	})
}

func TestSpawnAgent(t *testing.T) {
	t.Run("forwards prompt to provider", func(t *testing.T) {
		mp := newMockProvider("result")
		reg := tool.NewRegistry()
		tl := SubagentTools(mp, reg, nil)[0]

		result, err := tl.Execute(context.Background(), map[string]any{
			"prompt": "summarize this",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(mp.calls) != 1 {
			t.Fatalf("expected 1 Generate call, got %d", len(mp.calls))
		}

		call := mp.calls[0]
		if len(call.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(call.Messages))
		}
		if call.Messages[0].Role != provider.RoleUser {
			t.Errorf("expected user role, got %q", call.Messages[0].Role)
		}
		if call.Messages[0].Content != "summarize this" {
			t.Errorf("expected prompt %q, got %q", "summarize this", call.Messages[0].Content)
		}

		var out spawnAgentOutput
		if err := json.Unmarshal([]byte(result), &out); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if out.Result != "result" {
			t.Errorf("expected result %q, got %q", "result", out.Result)
		}
	})

	t.Run("uses default system prompt when omitted", func(t *testing.T) {
		mp := newMockProvider("ok")
		reg := tool.NewRegistry()
		tl := SubagentTools(mp, reg, nil)[0]

		_, err := tl.Execute(context.Background(), map[string]any{
			"prompt": "hello",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mp.calls[0].SystemPrompt != defaultSubagentSystemPrompt {
			t.Errorf("expected default system prompt, got %q", mp.calls[0].SystemPrompt)
		}
	})

	t.Run("uses custom system prompt when provided", func(t *testing.T) {
		mp := newMockProvider("ok")
		reg := tool.NewRegistry()
		tl := SubagentTools(mp, reg, nil)[0]

		_, err := tl.Execute(context.Background(), map[string]any{
			"prompt":        "hello",
			"system_prompt": "You are a translator.",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mp.calls[0].SystemPrompt != "You are a translator." {
			t.Errorf("expected custom system prompt, got %q", mp.calls[0].SystemPrompt)
		}
	})

	t.Run("uses default provider when model omitted", func(t *testing.T) {
		defaultMP := newMockProvider("from default")
		reg := tool.NewRegistry()
		tl := SubagentTools(defaultMP, reg, nil)[0]

		result, err := tl.Execute(context.Background(), map[string]any{
			"prompt": "hello",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(defaultMP.calls) != 1 {
			t.Fatalf("expected default provider to be called, got %d calls", len(defaultMP.calls))
		}

		var out spawnAgentOutput
		json.Unmarshal([]byte(result), &out)
		if out.Result != "from default" {
			t.Errorf("expected %q, got %q", "from default", out.Result)
		}
	})

	t.Run("resolves model via resolver when model provided", func(t *testing.T) {
		defaultMP := newMockProvider("from default")
		overrideMP := newMockProvider("from override")
		reg := tool.NewRegistry()

		resolver := func(_ context.Context, model string) (provider.Provider, error) {
			if model == "openai/gpt-4o-mini" {
				return overrideMP, nil
			}
			return nil, nil
		}

		tl := SubagentTools(defaultMP, reg, resolver)[0]
		result, err := tl.Execute(context.Background(), map[string]any{
			"prompt": "hello",
			"model":  "openai/gpt-4o-mini",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(defaultMP.calls) != 0 {
			t.Error("expected default provider NOT to be called")
		}
		if len(overrideMP.calls) != 1 {
			t.Fatalf("expected override provider to be called once, got %d", len(overrideMP.calls))
		}

		var out spawnAgentOutput
		json.Unmarshal([]byte(result), &out)
		if out.Result != "from override" {
			t.Errorf("expected %q, got %q", "from override", out.Result)
		}
	})

	t.Run("child excludes spawn_agent from tools", func(t *testing.T) {
		mp := newMockProvider("ok")

		// Create a registry with some tools including spawn_agent itself.
		dummyTool := tool.NewTool("dummy_tool", "A dummy tool",
			func(ctx context.Context, in struct{}) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		spawnTool := SubagentTools(mp, nil, nil)[0]

		reg := tool.NewRegistry(dummyTool, spawnTool)
		tl := SubagentTools(mp, reg, nil)[0]

		_, err := tl.Execute(context.Background(), map[string]any{
			"prompt": "hello",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		call := mp.calls[0]
		for _, t2 := range call.Tools {
			if t2.Name == "spawn_agent" {
				t.Error("spawn_agent should be excluded from child tools")
			}
		}

		// Verify dummy_tool IS present.
		found := false
		for _, t2 := range call.Tools {
			if t2.Name == "dummy_tool" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected dummy_tool in child tools")
		}
	})

	t.Run("explicit tools list restricts child tools", func(t *testing.T) {
		mp := newMockProvider("ok")

		dummyA := tool.NewTool("tool_a", "Tool A",
			func(ctx context.Context, in struct{}) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		dummyB := tool.NewTool("tool_b", "Tool B",
			func(ctx context.Context, in struct{}) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		dummyC := tool.NewTool("tool_c", "Tool C",
			func(ctx context.Context, in struct{}) (struct{}, error) {
				return struct{}{}, nil
			},
		)

		reg := tool.NewRegistry(dummyA, dummyB, dummyC)
		tl := SubagentTools(mp, reg, nil)[0]

		_, err := tl.Execute(context.Background(), map[string]any{
			"prompt": "hello",
			"tools":  []any{"tool_a", "tool_c"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		call := mp.calls[0]
		names := map[string]bool{}
		for _, td := range call.Tools {
			names[td.Name] = true
		}
		if !names["tool_a"] {
			t.Error("expected tool_a in child tools")
		}
		if !names["tool_c"] {
			t.Error("expected tool_c in child tools")
		}
		if names["tool_b"] {
			t.Error("tool_b should be excluded from child tools")
		}
	})

	t.Run("empty tools list gives child no tools", func(t *testing.T) {
		mp := newMockProvider("ok")

		dummyA := tool.NewTool("tool_a", "Tool A",
			func(ctx context.Context, in struct{}) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		reg := tool.NewRegistry(dummyA)
		tl := SubagentTools(mp, reg, nil)[0]

		_, err := tl.Execute(context.Background(), map[string]any{
			"prompt": "hello",
			"tools":  []any{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		call := mp.calls[0]
		if len(call.Tools) != 0 {
			t.Errorf("expected no tools, got %d", len(call.Tools))
		}
	})

	t.Run("unknown tool names are ignored", func(t *testing.T) {
		mp := newMockProvider("ok")

		dummyA := tool.NewTool("tool_a", "Tool A",
			func(ctx context.Context, in struct{}) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		reg := tool.NewRegistry(dummyA)
		tl := SubagentTools(mp, reg, nil)[0]

		_, err := tl.Execute(context.Background(), map[string]any{
			"prompt": "hello",
			"tools":  []any{"tool_a", "nonexistent_tool"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		call := mp.calls[0]
		if len(call.Tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(call.Tools))
		}
		if call.Tools[0].Name != "tool_a" {
			t.Errorf("expected tool_a, got %q", call.Tools[0].Name)
		}
	})

	t.Run("returns final text from last model message", func(t *testing.T) {
		mp := newMockProvider("final answer")
		reg := tool.NewRegistry()
		tl := SubagentTools(mp, reg, nil)[0]

		result, err := tl.Execute(context.Background(), map[string]any{
			"prompt": "question",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out spawnAgentOutput
		if err := json.Unmarshal([]byte(result), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out.Result != "final answer" {
			t.Errorf("expected %q, got %q", "final answer", out.Result)
		}
	})

	t.Run("propagates tool call callback with agent=subagent", func(t *testing.T) {
		// Create a provider that makes a tool call, then returns text.
		var callNum int
		childProvider := &funcProvider{fn: func(_ context.Context, params *provider.GenerateParams) (*provider.Response, error) {
			callNum++
			if callNum == 1 {
				return &provider.Response{
					Message: provider.Message{
						Role: provider.RoleModel,
						ToolCalls: []provider.ToolCall{
							{Name: "dummy_tool", ID: "t1", Args: map[string]any{"key": "val"}},
						},
					},
				}, nil
			}
			return &provider.Response{
				Message: provider.Message{Role: provider.RoleModel, Content: "done"},
			}, nil
		}}

		dummyTool := tool.NewTool("dummy_tool", "A dummy tool",
			func(ctx context.Context, in struct {
				Key string `json:"key"`
			}) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		reg := tool.NewRegistry(dummyTool)
		tl := SubagentTools(childProvider, reg, nil)[0]

		// Set parent callback in context.
		type callRecord struct {
			name, args, agent string
		}
		var recorded []callRecord
		ctx := provider.WithToolCallCallback(context.Background(), func(name, args, agent string) {
			recorded = append(recorded, callRecord{name, args, agent})
		})

		_, err := tl.Execute(ctx, map[string]any{"prompt": "do it"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(recorded) != 1 {
			t.Fatalf("expected 1 callback invocation, got %d", len(recorded))
		}
		if recorded[0].name != "dummy_tool" {
			t.Errorf("name = %q, want %q", recorded[0].name, "dummy_tool")
		}
		if recorded[0].agent != "subagent" {
			t.Errorf("agent = %q, want %q", recorded[0].agent, "subagent")
		}
	})

	t.Run("propagates custom name through callback", func(t *testing.T) {
		var callNum int
		childProvider := &funcProvider{fn: func(_ context.Context, params *provider.GenerateParams) (*provider.Response, error) {
			callNum++
			if callNum == 1 {
				return &provider.Response{
					Message: provider.Message{
						Role: provider.RoleModel,
						ToolCalls: []provider.ToolCall{
							{Name: "dummy_tool", ID: "t1", Args: map[string]any{"key": "val"}},
						},
					},
				}, nil
			}
			return &provider.Response{
				Message: provider.Message{Role: provider.RoleModel, Content: "done"},
			}, nil
		}}

		dummyTool := tool.NewTool("dummy_tool", "A dummy tool",
			func(ctx context.Context, in struct {
				Key string `json:"key"`
			}) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		reg := tool.NewRegistry(dummyTool)
		tl := SubagentTools(childProvider, reg, nil)[0]

		type callRecord struct {
			name, args, agent string
		}
		var recorded []callRecord
		ctx := provider.WithToolCallCallback(context.Background(), func(name, args, agent string) {
			recorded = append(recorded, callRecord{name, args, agent})
		})

		_, err := tl.Execute(ctx, map[string]any{"prompt": "do it", "name": "researcher"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(recorded) != 1 {
			t.Fatalf("expected 1 callback invocation, got %d", len(recorded))
		}
		if recorded[0].agent != "researcher" {
			t.Errorf("agent = %q, want %q", recorded[0].agent, "researcher")
		}
	})

	t.Run("defaults to subagent when name is omitted", func(t *testing.T) {
		var callNum int
		childProvider := &funcProvider{fn: func(_ context.Context, params *provider.GenerateParams) (*provider.Response, error) {
			callNum++
			if callNum == 1 {
				return &provider.Response{
					Message: provider.Message{
						Role: provider.RoleModel,
						ToolCalls: []provider.ToolCall{
							{Name: "dummy_tool", ID: "t1", Args: map[string]any{"key": "val"}},
						},
					},
				}, nil
			}
			return &provider.Response{
				Message: provider.Message{Role: provider.RoleModel, Content: "done"},
			}, nil
		}}

		dummyTool := tool.NewTool("dummy_tool", "A dummy tool",
			func(ctx context.Context, in struct {
				Key string `json:"key"`
			}) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		reg := tool.NewRegistry(dummyTool)
		tl := SubagentTools(childProvider, reg, nil)[0]

		type callRecord struct {
			name, args, agent string
		}
		var recorded []callRecord
		ctx := provider.WithToolCallCallback(context.Background(), func(name, args, agent string) {
			recorded = append(recorded, callRecord{name, args, agent})
		})

		_, err := tl.Execute(ctx, map[string]any{"prompt": "do it"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(recorded) != 1 {
			t.Fatalf("expected 1 callback invocation, got %d", len(recorded))
		}
		if recorded[0].agent != "subagent" {
			t.Errorf("agent = %q, want %q", recorded[0].agent, "subagent")
		}
	})

	t.Run("runs without error when no parent callback in context", func(t *testing.T) {
		mp := newMockProvider("ok")
		reg := tool.NewRegistry()
		tl := SubagentTools(mp, reg, nil)[0]

		// Use bare context without any callback set.
		_, err := tl.Execute(context.Background(), map[string]any{
			"prompt": "hello",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("concurrent subagents with different names show correct labels", func(t *testing.T) {
		// Each child provider invokes a tool, so the parent callback fires
		// with the correct agent name for each subagent.
		child := &funcProvider{fn: func(_ context.Context, params *provider.GenerateParams) (*provider.Response, error) {
			// First call: invoke dummy_tool; second call: return text.
			if len(params.Messages) == 1 {
				return &provider.Response{
					Message: provider.Message{
						Role: provider.RoleModel,
						ToolCalls: []provider.ToolCall{
							{Name: "dummy_tool", ID: "t1", Args: map[string]any{}},
						},
					},
				}, nil
			}
			return &provider.Response{
				Message: provider.Message{Role: provider.RoleModel, Content: "done"},
			}, nil
		}}

		dummyTool := tool.NewTool("dummy_tool", "A dummy tool",
			func(ctx context.Context, in struct{}) (struct{}, error) {
				return struct{}{}, nil
			},
		)
		reg := tool.NewRegistry(dummyTool)
		reg.Add(SubagentTools(child, reg, nil))

		// Parent provider issues 3 spawn_agent calls with different names.
		var parentCall int
		parent := &funcProvider{fn: func(_ context.Context, _ *provider.GenerateParams) (*provider.Response, error) {
			parentCall++
			if parentCall == 1 {
				return &provider.Response{
					Message: provider.Message{
						Role: provider.RoleModel,
						ToolCalls: []provider.ToolCall{
							{Name: "spawn_agent", ID: "a", Args: map[string]any{"prompt": "task 0", "name": "researcher"}},
							{Name: "spawn_agent", ID: "b", Args: map[string]any{"prompt": "task 1", "name": "summarizer"}},
							{Name: "spawn_agent", ID: "c", Args: map[string]any{"prompt": "task 2"}},
						},
					},
				}, nil
			}
			return &provider.Response{
				Message: provider.Message{Role: provider.RoleModel, Content: "all done"},
			}, nil
		}}

		type callRecord struct {
			name, agent string
		}
		var mu sync.Mutex
		var recorded []callRecord
		onToolCall := func(name, args, agent string) {
			mu.Lock()
			recorded = append(recorded, callRecord{name, agent})
			mu.Unlock()
		}

		ctx := provider.WithToolCallCallback(context.Background(), onToolCall)
		_, _, err := provider.RunAgentLoop(
			ctx, parent, "sys",
			[]provider.Message{{Role: provider.RoleUser, Content: "run tasks"}},
			reg.Tools(), reg.Executor(), onToolCall, nil,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Collect agent labels from callback records for dummy_tool calls
		// (spawn_agent itself also fires callbacks, filter to child tool calls).
		mu.Lock()
		defer mu.Unlock()

		agentNames := map[string]bool{}
		for _, r := range recorded {
			if r.name == "dummy_tool" {
				agentNames[r.agent] = true
			}
		}

		if !agentNames["researcher"] {
			t.Error("expected callback with agent=researcher")
		}
		if !agentNames["summarizer"] {
			t.Error("expected callback with agent=summarizer")
		}
		if !agentNames["subagent"] {
			t.Error("expected callback with agent=subagent (default for unnamed)")
		}
	})

	t.Run("parallel spawn_agent calls execute concurrently", func(t *testing.T) {
		const agentDelay = 100 * time.Millisecond

		// Track when each child agent starts to verify concurrency.
		var mu sync.Mutex
		var childStarts []time.Time

		child := &funcProvider{fn: func(_ context.Context, params *provider.GenerateParams) (*provider.Response, error) {
			mu.Lock()
			childStarts = append(childStarts, time.Now())
			mu.Unlock()
			time.Sleep(agentDelay)
			return &provider.Response{
				Message: provider.Message{
					Role:    provider.RoleModel,
					Content: "done: " + params.Messages[0].Content,
				},
			}, nil
		}}

		reg := tool.NewRegistry()
		reg.Add(SubagentTools(child, reg, nil))

		// Parent provider: first call returns 3 spawn_agent tool calls,
		// second call returns final text.
		var parentCall int
		parent := &funcProvider{fn: func(_ context.Context, _ *provider.GenerateParams) (*provider.Response, error) {
			parentCall++
			if parentCall == 1 {
				return &provider.Response{
					Message: provider.Message{
						Role: provider.RoleModel,
						ToolCalls: []provider.ToolCall{
							{Name: "spawn_agent", ID: "a", Args: map[string]any{"prompt": "task 0"}},
							{Name: "spawn_agent", ID: "b", Args: map[string]any{"prompt": "task 1"}},
							{Name: "spawn_agent", ID: "c", Args: map[string]any{"prompt": "task 2"}},
						},
					},
				}, nil
			}
			return &provider.Response{
				Message: provider.Message{Role: provider.RoleModel, Content: "all done"},
			}, nil
		}}

		before := time.Now()
		msgs, _, err := provider.RunAgentLoop(
			context.Background(), parent, "sys",
			[]provider.Message{{Role: provider.RoleUser, Content: "run tasks"}},
			reg.Tools(), reg.Executor(), nil, nil,
		)
		elapsed := time.Since(before)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify concurrency: if sequential, total >= 300ms; if concurrent, ~100ms.
		if elapsed >= 3*agentDelay {
			t.Errorf("expected concurrent execution (<%v), took %v", 3*agentDelay, elapsed)
		}

		// Child start times should be near-simultaneous.
		mu.Lock()
		starts := append([]time.Time{}, childStarts...)
		mu.Unlock()
		if len(starts) != 3 {
			t.Fatalf("expected 3 child starts, got %d", len(starts))
		}
		sort.Slice(starts, func(i, j int) bool { return starts[i].Before(starts[j]) })
		if gap := starts[2].Sub(starts[0]); gap > 50*time.Millisecond {
			t.Errorf("child agents not concurrent: start time spread = %v, want < 50ms", gap)
		}

		// Verify all results are correctly returned to the parent.
		// Expected messages: user, model(tool calls), 3×tool, model(final).
		var results []string
		for _, m := range msgs {
			if m.Role == provider.RoleTool {
				var out spawnAgentOutput
				if err := json.Unmarshal([]byte(m.Content), &out); err != nil {
					t.Fatalf("unmarshal tool result: %v", err)
				}
				results = append(results, out.Result)
			}
		}
		sort.Strings(results)
		want := []string{"done: task 0", "done: task 1", "done: task 2"}
		if len(results) != len(want) {
			t.Fatalf("expected %d results, got %d", len(want), len(results))
		}
		for i := range want {
			if results[i] != want[i] {
				t.Errorf("result[%d] = %q, want %q", i, results[i], want[i])
			}
		}
	})
}
