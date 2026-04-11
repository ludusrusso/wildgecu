package tools

import (
	"context"
	"encoding/json"
	"testing"

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
}
