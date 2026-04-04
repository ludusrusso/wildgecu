package agent

import (
	"context"
	"errors"
	"testing"

	"wildgecu/pkg/home"
	"wildgecu/pkg/provider"
)

// --- LoadSoul ---

func TestLoadSoul_Exists(t *testing.T) {
	h := home.NewTmpHome(t)
	if err := h.Soul().Write("I am a helpful agent."); err != nil {
		t.Fatalf("setup: %v", err)
	}

	content, err := LoadSoul(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "I am a helpful agent." {
		t.Fatalf("got %q, want %q", content, "I am a helpful agent.")
	}
}

func TestLoadSoul_NotExists(t *testing.T) {
	h := home.NewTmpHome(t)

	content, err := LoadSoul(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Fatalf("got %q, want empty string", content)
	}
}

// --- LoadMemory ---

func TestLoadMemory_Exists(t *testing.T) {
	h := home.NewTmpHome(t)
	if err := h.Memory().Write("some memory"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	content, err := LoadMemory(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "some memory" {
		t.Fatalf("got %q, want %q", content, "some memory")
	}
}

func TestLoadMemory_NotExists(t *testing.T) {
	h := home.NewTmpHome(t)

	content, err := LoadMemory(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Fatalf("got %q, want empty string", content)
	}
}

// --- BootstrapConfig ---

func TestBootstrapConfig_WritesSoul(t *testing.T) {
	h := home.NewTmpHome(t)
	var soulContent string

	cfg := BootstrapConfig(context.Background(), nil, h, &soulContent)

	tc := provider.ToolCall{
		ID:   "1",
		Name: "write_soul",
		Args: map[string]any{"content": "My soul content"},
	}

	result, err := cfg.Executor(context.Background(), tc)
	if !errors.Is(err, provider.ErrDone) {
		t.Fatalf("expected provider.ErrDone, got %v", err)
	}
	if result != `{"status":"ok"}` {
		t.Fatalf("unexpected result: %s", result)
	}
	if soulContent != "My soul content" {
		t.Fatalf("soulContent = %q, want %q", soulContent, "My soul content")
	}
	data, err := h.Soul().Get()
	if err != nil {
		t.Fatalf("SOUL.md not persisted: %v", err)
	}
	if data != "My soul content" {
		t.Fatalf("persisted content = %q, want %q", data, "My soul content")
	}
}

func TestBootstrapConfig_EmptyContent(t *testing.T) {
	h := home.NewTmpHome(t)
	var soulContent string

	cfg := BootstrapConfig(context.Background(), nil, h, &soulContent)

	tc := provider.ToolCall{
		ID:   "1",
		Name: "write_soul",
		Args: map[string]any{"content": ""},
	}

	_, err := cfg.Executor(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestBootstrapConfig_UnknownTool(t *testing.T) {
	h := home.NewTmpHome(t)
	var soulContent string

	cfg := BootstrapConfig(context.Background(), nil, h, &soulContent)

	tc := provider.ToolCall{
		ID:   "1",
		Name: "unknown_tool",
		Args: map[string]any{},
	}

	result, err := cfg.Executor(context.Background(), tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"error": "unknown tool: unknown_tool"}` {
		t.Fatalf("unexpected result: %s", result)
	}
}

// --- BuildSystemPrompt ---

func TestBuildSystemPrompt_WithSoul(t *testing.T) {
	ws := home.NewTmpHome(t)

	prompt := BuildSystemPrompt(ws, "I am a test soul.", "")
	if !contains(prompt, "# Agent Soul") {
		t.Fatal("expected soul section in prompt")
	}
	if !contains(prompt, "I am a test soul.") {
		t.Fatal("expected soul content in prompt")
	}
}

func TestBuildSystemPrompt_WithUserPrefs(t *testing.T) {
	ws := home.NewTmpHome(t)
	if err := ws.User().Write("Prefer Go."); err != nil {
		t.Fatalf("setup: %v", err)
	}

	prompt := BuildSystemPrompt(ws, "soul", "")
	if !contains(prompt, "# User Preferences") {
		t.Fatal("expected user preferences section in prompt")
	}
	if !contains(prompt, "Prefer Go.") {
		t.Fatal("expected user prefs content in prompt")
	}
}

func TestBuildSystemPrompt_NoUserPrefs(t *testing.T) {
	ws := home.NewTmpHome(t)

	prompt := BuildSystemPrompt(ws, "soul", "")
	if contains(prompt, "# User Preferences") {
		t.Fatal("did not expect user preferences section in prompt")
	}
}

func TestBuildSystemPrompt_NilWorkspace(t *testing.T) {
	prompt := BuildSystemPrompt(nil, "soul", "")
	if contains(prompt, "# User Preferences") {
		t.Fatal("did not expect user preferences section with nil workspace")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
