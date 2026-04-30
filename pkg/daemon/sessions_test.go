package daemon

import (
	"context"
	"sync"
	"testing"

	"github.com/ludusrusso/wildgecu/pkg/agent"
	"github.com/ludusrusso/wildgecu/pkg/home"
	"github.com/ludusrusso/wildgecu/pkg/provider"
	"github.com/ludusrusso/wildgecu/pkg/session"
)

// capturingProvider records the last system prompt and user message it received.
type capturingProvider struct {
	lastSystemPrompt string
	lastUserMessage  string
}

func (p *capturingProvider) Generate(_ context.Context, params *provider.GenerateParams) (*provider.Response, error) {
	p.lastSystemPrompt = params.SystemPrompt
	for i := len(params.Messages) - 1; i >= 0; i-- {
		if params.Messages[i].Role == provider.RoleUser {
			p.lastUserMessage = params.Messages[i].Content
			break
		}
	}
	return &provider.Response{
		Message: provider.Message{Role: "assistant", Content: "ok"},
	}, nil
}

// fakeProvider returns a canned response without calling any real LLM.
type fakeProvider struct{}

func (fakeProvider) Generate(_ context.Context, _ *provider.GenerateParams) (*provider.Response, error) {
	return &provider.Response{
		Message: provider.Message{Role: "assistant", Content: "ok"},
	}, nil
}

func newTestSessionManager(t *testing.T) *SessionManager {
	t.Helper()
	h, err := home.New(t.TempDir())
	if err != nil {
		t.Fatalf("home.New: %v", err)
	}
	return &SessionManager{
		agentCfg: agent.Config{
			Provider: fakeProvider{},
			Home:     h,
		},
		chatCfg: &session.Config{
			Provider:    fakeProvider{},
			WelcomeText: "hello",
		},
		sessions: make(map[string]*ManagedSession),
	}
}

func TestReset(t *testing.T) {
	t.Run("returns new session with different ID", func(t *testing.T) {
		sm := newTestSessionManager(t)
		old := sm.Create()
		oldID := old.ID

		// Add a message so the old session is non-empty.
		old.messages = append(old.messages, provider.Message{Role: "user", Content: "hi"})

		newSess, err := sm.Reset(context.Background(), oldID)
		if err != nil {
			t.Fatalf("Reset() error: %v", err)
		}
		if newSess.ID == oldID {
			t.Error("expected new session to have a different ID")
		}
	})

	t.Run("old session is removed", func(t *testing.T) {
		sm := newTestSessionManager(t)
		old := sm.Create()
		oldID := old.ID

		_, err := sm.Reset(context.Background(), oldID)
		if err != nil {
			t.Fatalf("Reset() error: %v", err)
		}
		if sm.Get(oldID) != nil {
			t.Error("expected old session to be removed")
		}
	})

	t.Run("new session is retrievable", func(t *testing.T) {
		sm := newTestSessionManager(t)
		old := sm.Create()

		newSess, err := sm.Reset(context.Background(), old.ID)
		if err != nil {
			t.Fatalf("Reset() error: %v", err)
		}
		if sm.Get(newSess.ID) == nil {
			t.Error("expected new session to be retrievable")
		}
	})

	t.Run("new session has fresh messages", func(t *testing.T) {
		sm := newTestSessionManager(t)
		sm.chatCfg.InitialMessages = []provider.Message{
			{Role: "system", Content: "You are helpful."},
		}
		old := sm.Create()
		// Simulate conversation history.
		old.messages = append(old.messages,
			provider.Message{Role: "user", Content: "hello"},
			provider.Message{Role: "assistant", Content: "hi there"},
		)

		newSess, err := sm.Reset(context.Background(), old.ID)
		if err != nil {
			t.Fatalf("Reset() error: %v", err)
		}
		// New session should only have the initial messages, not the old conversation.
		messages := newSess.Messages()
		if len(messages) != 1 {
			t.Errorf("expected 1 initial message, got %d", len(messages))
		}
		if messages[0].Content != "You are helpful." {
			t.Errorf("expected initial system message, got %q", messages[0].Content)
		}
	})

	t.Run("error on unknown session", func(t *testing.T) {
		sm := newTestSessionManager(t)

		_, err := sm.Reset(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown session")
		}
	})
}

func TestRunSkillTurnStream(t *testing.T) {
	newSMWithCapture := func(basePrompt string) (*SessionManager, *capturingProvider) {
		cp := &capturingProvider{}
		sm := &SessionManager{
			chatCfg: &session.Config{
				Provider:     cp,
				SystemPrompt: basePrompt,
				WelcomeText:  "hello",
			},
			sessions: make(map[string]*ManagedSession),
		}
		return sm, cp
	}

	t.Run("injects skill content into system prompt", func(t *testing.T) {
		sm, cp := newSMWithCapture("base prompt")
		sess := sm.Create()

		if _, err := sm.RunSkillTurnStream(context.Background(), sess.ID, "skill instructions", "do the thing", nil, nil, nil, nil); err != nil {
			t.Fatalf("RunSkillTurnStream() error: %v", err)
		}

		want := "base prompt\n\nskill instructions"
		if cp.lastSystemPrompt != want {
			t.Errorf("SystemPrompt = %q, want %q", cp.lastSystemPrompt, want)
		}
	})

	t.Run("does not modify system prompt when skill content is empty", func(t *testing.T) {
		sm, cp := newSMWithCapture("base prompt")
		sess := sm.Create()

		if _, err := sm.RunSkillTurnStream(context.Background(), sess.ID, "", "do the thing", nil, nil, nil, nil); err != nil {
			t.Fatalf("RunSkillTurnStream() error: %v", err)
		}

		if cp.lastSystemPrompt != "base prompt" {
			t.Errorf("SystemPrompt = %q, want %q", cp.lastSystemPrompt, "base prompt")
		}
	})

	t.Run("passes user input as user message", func(t *testing.T) {
		sm, cp := newSMWithCapture("base prompt")
		sess := sm.Create()

		if _, err := sm.RunSkillTurnStream(context.Background(), sess.ID, "skill instructions", "review main.go", nil, nil, nil, nil); err != nil {
			t.Fatalf("RunSkillTurnStream() error: %v", err)
		}

		if cp.lastUserMessage != "review main.go" {
			t.Errorf("lastUserMessage = %q, want %q", cp.lastUserMessage, "review main.go")
		}
	})

	t.Run("skill content does not persist to subsequent turns", func(t *testing.T) {
		sm, cp := newSMWithCapture("base prompt")
		sess := sm.Create()

		if _, err := sm.RunSkillTurnStream(context.Background(), sess.ID, "skill instructions", "do the thing", nil, nil, nil, nil); err != nil {
			t.Fatalf("RunSkillTurnStream() error: %v", err)
		}
		if _, err := sm.RunTurnStream(context.Background(), sess.ID, "follow up", nil, nil, nil, nil); err != nil {
			t.Fatalf("RunTurnStream() error: %v", err)
		}

		if cp.lastSystemPrompt != "base prompt" {
			t.Errorf("after regular turn, SystemPrompt = %q, want %q", cp.lastSystemPrompt, "base prompt")
		}
	})
}

// toolCallProvider makes one tool call then returns text.
type toolCallProvider struct {
	callNum int
}

func (p *toolCallProvider) Generate(_ context.Context, params *provider.GenerateParams) (*provider.Response, error) {
	p.callNum++
	if p.callNum == 1 {
		return &provider.Response{
			Message: provider.Message{
				Role: provider.RoleModel,
				ToolCalls: []provider.ToolCall{
					{Name: "test_tool", ID: "t1", Args: map[string]any{"x": 1}},
				},
			},
		}, nil
	}
	return &provider.Response{
		Message: provider.Message{Role: provider.RoleModel, Content: "done"},
	}, nil
}

func TestRunTurnStream(t *testing.T) {
	t.Run("callback is accessible from context during tool execution", func(t *testing.T) {
		// This is the critical test: when RunTurnStream sets cfg.OnToolCall,
		// tool executors must be able to retrieve it via GetToolCallCallback(ctx).
		// This is the path spawn_agent uses to surface subagent tool calls.
		var contextCallbackNil bool
		executor := func(ctx context.Context, tc provider.ToolCall) (string, error) {
			cb := provider.GetToolCallCallback(ctx)
			contextCallbackNil = (cb == nil)
			return "ok", nil
		}

		p := &toolCallProvider{}
		sm := &SessionManager{
			chatCfg: &session.Config{
				Provider:     p,
				SystemPrompt: "test",
				Tools:        []provider.Tool{{Name: "test_tool"}},
				Executor:     executor,
			},
			sessions: make(map[string]*ManagedSession),
		}
		sess := sm.Create()

		var mu sync.Mutex
		var toolCalls []string
		onToolCall := func(name, args, agent string) {
			mu.Lock()
			toolCalls = append(toolCalls, name)
			mu.Unlock()
		}

		_, err := sm.RunTurnStream(context.Background(), sess.ID, "hello", nil, onToolCall, nil, nil)
		if err != nil {
			t.Fatalf("RunTurnStream error: %v", err)
		}

		if contextCallbackNil {
			t.Error("GetToolCallCallback(ctx) returned nil during tool execution — spawn_agent cannot propagate subagent tool calls")
		}

		mu.Lock()
		defer mu.Unlock()
		if len(toolCalls) == 0 {
			t.Error("expected onToolCall to be invoked at least once")
		}
	})
}
