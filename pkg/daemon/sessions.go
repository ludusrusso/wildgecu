package daemon

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ludusrusso/wildgecu/pkg/agent"
	"github.com/ludusrusso/wildgecu/pkg/agent/tools"
	"github.com/ludusrusso/wildgecu/pkg/provider"
	"github.com/ludusrusso/wildgecu/pkg/session"
	"github.com/ludusrusso/wildgecu/pkg/todo"
	"github.com/ludusrusso/wildgecu/x/container"

	"github.com/google/uuid"
)

// SessionManager owns all chat sessions and delegates to session.RunTurnStream.
type SessionManager struct {
	agentCfg  agent.Config
	chatCfg   *session.Config
	container *container.Container
	sessions  map[string]*ManagedSession
	mu        sync.RWMutex
}

// ManagedSession holds the state for a single chat session. Read messages and
// todos via Messages() and Todos(); the unexported fields require mu.
type ManagedSession struct {
	ID string
	// messages is append-only: a turn replaces the slice header but never
	// mutates existing elements, so Messages() can hand out a shallow copy
	// that is safe to read without further locking.
	messages    []provider.Message
	todos       *todo.List
	cfg         *session.Config // per-session config (nil = use shared chatCfg)
	welcomeText string          // per-session welcome text
	mu          sync.Mutex
	createdAt   time.Time
	cancel      context.CancelFunc
	cancelMu    sync.Mutex
}

// Messages returns a shallow copy of the session's conversation history.
func (s *ManagedSession) Messages() []provider.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.messages)
}

// Todos returns the session's todo list. The list is internally thread-safe.
func (s *ManagedSession) Todos() *todo.List {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.todos
}

// cancelInflight cancels the current turn if one is running.
func (s *ManagedSession) cancelInflight() {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

// NewSessionManager calls agent.Prepare to build the shared session.Config.
func NewSessionManager(ctx context.Context, agentCfg agent.Config, ctr *container.Container) (*SessionManager, error) {
	chatCfg, dbg, err := agent.Prepare(ctx, agentCfg)
	if err != nil {
		return nil, fmt.Errorf("agent prepare: %w", err)
	}
	// The debug logger is owned by the session manager lifetime.
	// We don't close it here; it stays open for the daemon's lifetime.
	_ = dbg

	return &SessionManager{
		agentCfg:  agentCfg,
		chatCfg:   chatCfg,
		container: ctr,
		sessions:  make(map[string]*ManagedSession),
	}, nil
}

// Create creates a new session with a unique ID.
func (sm *SessionManager) Create() *ManagedSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess := &ManagedSession{
		ID:        uuid.New().String(),
		messages:  append([]provider.Message{}, sm.chatCfg.InitialMessages...),
		todos:     todo.New(),
		createdAt: time.Now(),
	}
	sm.sessions[sess.ID] = sess
	return sess
}

// CreateSession creates a new session and returns its ID.
// This satisfies the telegram.SessionProvider interface.
func (sm *SessionManager) CreateSession() string {
	return sm.Create().ID
}

// CreateCode creates a new code-mode session with file tools and workDir-scoped bash.
func (sm *SessionManager) CreateCode(workDir string) (*ManagedSession, error) {
	codeCfg, _, err := agent.PrepareCode(context.Background(), sm.agentCfg, workDir)
	if err != nil {
		return nil, err
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sess := &ManagedSession{
		ID:          uuid.New().String(),
		messages:    append([]provider.Message{}, codeCfg.InitialMessages...),
		cfg:         codeCfg,
		welcomeText: codeCfg.WelcomeText,
		createdAt:   time.Now(),
		todos:       todo.New(),
	}
	sm.sessions[sess.ID] = sess
	return sess, nil
}

// CreateWithModel creates a chat session using a specific model override.
func (sm *SessionManager) CreateWithModel(model string) (*ManagedSession, error) {
	p, err := sm.container.Get(context.Background(), model)
	if err != nil {
		return nil, fmt.Errorf("resolve model %q: %w", model, err)
	}
	overrideCfg := sm.agentCfg
	overrideCfg.Provider = p
	chatCfg, _, err := agent.Prepare(context.Background(), overrideCfg)
	if err != nil {
		return nil, fmt.Errorf("prepare session for model %q: %w", model, err)
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sess := &ManagedSession{
		ID:          uuid.New().String(),
		messages:    append([]provider.Message{}, chatCfg.InitialMessages...),
		cfg:         chatCfg,
		welcomeText: chatCfg.WelcomeText,
		createdAt:   time.Now(),
		todos:       todo.New(),
	}
	sm.sessions[sess.ID] = sess
	return sess, nil
}

// CreateCodeWithModel creates a code-mode session using a specific model override.
func (sm *SessionManager) CreateCodeWithModel(workDir, model string) (*ManagedSession, error) {
	p, err := sm.container.Get(context.Background(), model)
	if err != nil {
		return nil, fmt.Errorf("resolve model %q: %w", model, err)
	}
	overrideCfg := sm.agentCfg
	overrideCfg.Provider = p
	codeCfg, _, err := agent.PrepareCode(context.Background(), overrideCfg, workDir)
	if err != nil {
		return nil, err
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sess := &ManagedSession{
		ID:          uuid.New().String(),
		messages:    append([]provider.Message{}, codeCfg.InitialMessages...),
		cfg:         codeCfg,
		welcomeText: codeCfg.WelcomeText,
		createdAt:   time.Now(),
		todos:       todo.New(),
	}
	sm.sessions[sess.ID] = sess
	return sess, nil
}

// Get returns the session with the given ID, or nil.
func (sm *SessionManager) Get(id string) *ManagedSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[id]
}

// Close cancels any in-flight turn, finalizes the session's memory on a
// stable view of the message history, and removes it from the manager map.
func (sm *SessionManager) Close(ctx context.Context, id string) {
	sm.mu.RLock()
	sess, ok := sm.sessions[id]
	sm.mu.RUnlock()
	if !ok {
		return
	}

	// Cancel first so a running turn releases sess.mu.
	sess.cancelInflight()

	// Hold sess.mu across Finalize so the message slice can't change mid-call.
	sess.mu.Lock()
	_ = agent.Finalize(ctx, sm.agentCfg, sess.messages)
	sess.mu.Unlock()

	sm.mu.Lock()
	delete(sm.sessions, id)
	sm.mu.Unlock()
}

// Reset closes the given session (with finalize) and creates a fresh one.
func (sm *SessionManager) Reset(ctx context.Context, id string) (*ManagedSession, error) {
	if sm.Get(id) == nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	sm.Close(ctx, id)
	return sm.Create(), nil
}

// ResetSession resets a session and returns the new session ID.
// This satisfies the telegram.SessionProvider interface.
func (sm *SessionManager) ResetSession(ctx context.Context, id string) (string, error) {
	sess, err := sm.Reset(ctx, id)
	if err != nil {
		return "", err
	}
	return sess.ID, nil
}

// Interrupt cancels the current turn if one is running.
func (sm *SessionManager) Interrupt(id string) {
	sess := sm.Get(id)
	if sess == nil {
		return
	}
	sess.cancelInflight()
}

// WelcomeText returns the configured welcome text.
func (sm *SessionManager) WelcomeText() string {
	return sm.chatCfg.WelcomeText
}

// OnChunkFunc is called for each streaming text chunk.
type OnChunkFunc func(chunk string)

// OnToolCallFunc is called when the agent invokes a tool.
// The agent parameter identifies the source (empty for the parent agent).
type OnToolCallFunc func(name, args, agent string)

// OnInformFunc is called when the agent sends a status message to the user.
type OnInformFunc func(message string)

// OnTodoSnapshotFunc is called after each todo_create or todo_update tool
// executes, carrying a defensive copy of the session's current todo list.
type OnTodoSnapshotFunc func(items []todo.Item)

// RunTurnStream runs a single conversational turn with streaming callbacks.
// It locks the session for the duration. onTodoSnapshot may be nil (Telegram
// path doesn't subscribe to live todo updates).
func (sm *SessionManager) RunTurnStream(ctx context.Context, id, input string, onChunk OnChunkFunc, onToolCall OnToolCallFunc, onInform OnInformFunc, onTodoSnapshot OnTodoSnapshotFunc) (string, error) {
	return sm.runTurnInternal(ctx, id, input, "", onChunk, onToolCall, onInform, onTodoSnapshot)
}

// RunSkillTurnStream runs a streaming LLM turn with skill content injected as
// additional system context. The skill content is appended to the session's
// system prompt, and userInput becomes the user message.
func (sm *SessionManager) RunSkillTurnStream(ctx context.Context, id, skillContent, userInput string, onChunk OnChunkFunc, onToolCall OnToolCallFunc, onInform OnInformFunc, onTodoSnapshot OnTodoSnapshotFunc) (string, error) {
	return sm.runTurnInternal(ctx, id, userInput, skillContent, onChunk, onToolCall, onInform, onTodoSnapshot)
}

// runTurnInternal is the shared implementation for RunTurnStream and
// RunSkillTurnStream. When extraSystem is non-empty it is appended to the
// session's system prompt.
func (sm *SessionManager) runTurnInternal(ctx context.Context, id, input, extraSystem string, onChunk OnChunkFunc, onToolCall OnToolCallFunc, onInform OnInformFunc, onTodoSnapshot OnTodoSnapshotFunc) (string, error) {
	sess := sm.Get(id)
	if sess == nil {
		return "", fmt.Errorf("session not found: %s", id)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	sess.cancelMu.Lock()
	sess.cancel = cancel
	sess.cancelMu.Unlock()
	defer func() {
		sess.cancelMu.Lock()
		sess.cancel = nil
		sess.cancelMu.Unlock()
		cancel()
	}()

	// Use per-session config if available, otherwise shared chatCfg.
	baseCfg := sm.chatCfg
	if sess.cfg != nil {
		baseCfg = sess.cfg
	}
	cfg := *baseCfg
	if extraSystem != "" {
		cfg.SystemPrompt = cfg.SystemPrompt + "\n\n" + extraSystem
	}
	if onToolCall != nil {
		cfg.OnToolCall = provider.ToolCallCallback(onToolCall)
	}
	if sess.todos != nil {
		todos := sess.todos
		cfg.RequestReminder = func() string { return todos.RenderSystemReminder() }
		ctx = todo.WithList(ctx, todos)
		if onTodoSnapshot != nil && cfg.Executor != nil {
			inner := cfg.Executor
			cfg.Executor = func(ctx context.Context, tc provider.ToolCall) (string, error) {
				result, err := inner(ctx, tc)
				if tc.Name == tools.TodoCreateName || tc.Name == tools.TodoUpdateName {
					onTodoSnapshot(todos.Snapshot())
				}
				return result, err
			}
		}
	}

	chunkCb := func(chunk string) {
		if onChunk != nil {
			onChunk(chunk)
		}
	}

	ctx = provider.WithInformFunc(ctx, func(msg string) {
		if onInform != nil {
			onInform(msg)
		}
	})

	updated, resp, err := session.RunTurnStream(ctx, &cfg, sess.messages, input, chunkCb)
	if err != nil {
		return "", err
	}

	sess.messages = updated
	return resp.Message.Content, nil
}

// RunSkillTurnStreamRaw is like RunSkillTurnStream but uses plain function
// types, making it usable as a telegram.SessionProvider method.
func (sm *SessionManager) RunSkillTurnStreamRaw(ctx context.Context, id, skillContent, userInput string, onChunk func(string), onToolCall func(string, string, string), onInform func(string)) (string, error) {
	var chunkCb OnChunkFunc
	if onChunk != nil {
		chunkCb = OnChunkFunc(onChunk)
	}
	var toolCb OnToolCallFunc
	if onToolCall != nil {
		toolCb = OnToolCallFunc(onToolCall)
	}
	var informCb OnInformFunc
	if onInform != nil {
		informCb = OnInformFunc(onInform)
	}
	return sm.RunSkillTurnStream(ctx, id, skillContent, userInput, chunkCb, toolCb, informCb, nil)
}

// RunTurnStreamRaw is like RunTurnStream but uses plain function types instead
// of named callback types, making it usable as a telegram.SessionProvider method.
func (sm *SessionManager) RunTurnStreamRaw(ctx context.Context, id, input string, onChunk func(string), onToolCall func(string, string, string), onInform func(string)) (string, error) {
	var chunkCb OnChunkFunc
	if onChunk != nil {
		chunkCb = OnChunkFunc(onChunk)
	}
	var toolCb OnToolCallFunc
	if onToolCall != nil {
		toolCb = OnToolCallFunc(onToolCall)
	}
	var informCb OnInformFunc
	if onInform != nil {
		informCb = OnInformFunc(onInform)
	}
	return sm.RunTurnStream(ctx, id, input, chunkCb, toolCb, informCb, nil)
}

