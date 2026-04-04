package daemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"wildgecu/pkg/agent"
	"wildgecu/pkg/provider"
	"wildgecu/pkg/session"

	"github.com/google/uuid"
)

// SessionManager owns all chat sessions and delegates to session.RunTurnStream.
type SessionManager struct {
	agentCfg agent.Config
	chatCfg  *session.Config
	sessions map[string]*ManagedSession
	mu       sync.RWMutex
}

// ManagedSession holds the state for a single chat session.
type ManagedSession struct {
	ID          string
	Messages    []provider.Message
	cfg         *session.Config // per-session config (nil = use shared chatCfg)
	welcomeText string          // per-session welcome text
	mu          sync.Mutex
	createdAt   time.Time
	cancel      context.CancelFunc
	cancelMu    sync.Mutex
}

// NewSessionManager calls agent.Prepare to build the shared session.Config.
func NewSessionManager(ctx context.Context, agentCfg agent.Config) (*SessionManager, error) {
	chatCfg, dbg, err := agent.Prepare(ctx, agentCfg)
	if err != nil {
		return nil, fmt.Errorf("agent prepare: %w", err)
	}
	// The debug logger is owned by the session manager lifetime.
	// We don't close it here; it stays open for the daemon's lifetime.
	_ = dbg

	return &SessionManager{
		agentCfg: agentCfg,
		chatCfg:  chatCfg,
		sessions: make(map[string]*ManagedSession),
	}, nil
}

// Create creates a new session with a unique ID.
func (sm *SessionManager) Create() *ManagedSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess := &ManagedSession{
		ID:        uuid.New().String(),
		Messages:  append([]provider.Message{}, sm.chatCfg.InitialMessages...),
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
		Messages:    append([]provider.Message{}, codeCfg.InitialMessages...),
		cfg:         codeCfg,
		welcomeText: codeCfg.WelcomeText,
		createdAt:   time.Now(),
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

// Close finalizes the session (updates memory) and removes it.
func (sm *SessionManager) Close(ctx context.Context, id string) {
	sm.mu.Lock()
	sess, ok := sm.sessions[id]
	if ok {
		delete(sm.sessions, id)
	}
	sm.mu.Unlock()

	if !ok || len(sess.Messages) == 0 {
		return
	}

	// Best-effort finalize (update memory).
	_ = agent.Finalize(ctx, sm.agentCfg, sess.Messages)
}


// Interrupt cancels the current turn if one is running.
func (sm *SessionManager) Interrupt(id string) {
	sess := sm.Get(id)
	if sess == nil {
		return
	}
	sess.cancelMu.Lock()
	defer sess.cancelMu.Unlock()
	if sess.cancel != nil {
		sess.cancel()
	}
}

// WelcomeText returns the configured welcome text.
func (sm *SessionManager) WelcomeText() string {
	return sm.chatCfg.WelcomeText
}

// OnChunkFunc is called for each streaming text chunk.
type OnChunkFunc func(chunk string)

// OnToolCallFunc is called when the agent invokes a tool.
type OnToolCallFunc func(name string, args string)

// RunTurnStream runs a single conversational turn with streaming callbacks.
// It locks the session for the duration.
func (sm *SessionManager) RunTurnStream(ctx context.Context, id, input string, onChunk OnChunkFunc, onToolCall OnToolCallFunc) (string, error) {
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
	cfg.OnToolCall = func(tc provider.ToolCall) {
		if onToolCall != nil {
			onToolCall(tc.Name, formatToolArgs(tc.Args, 100))
		}
	}

	chunkCb := func(chunk string) {
		if onChunk != nil {
			onChunk(chunk)
		}
	}

	updated, resp, err := session.RunTurnStream(ctx, &cfg, sess.Messages, input, chunkCb)
	if err != nil {
		return "", err
	}

	sess.Messages = updated
	return resp.Message.Content, nil
}

// RunTurnStreamRaw is like RunTurnStream but uses plain function types instead
// of named callback types, making it usable as a telegram.SessionProvider method.
func (sm *SessionManager) RunTurnStreamRaw(ctx context.Context, id, input string, onChunk func(string), onToolCall func(string, string)) (string, error) {
	var chunkCb OnChunkFunc
	if onChunk != nil {
		chunkCb = OnChunkFunc(onChunk)
	}
	var toolCb OnToolCallFunc
	if onToolCall != nil {
		toolCb = OnToolCallFunc(onToolCall)
	}
	return sm.RunTurnStream(ctx, id, input, chunkCb, toolCb)
}

// formatToolArgs formats a tool call's args map into a compact string.
func formatToolArgs(args map[string]any, maxLen int) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s: %v", k, v))
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	if len(result) > maxLen {
		result = result[:maxLen] + "..."
	}
	return result
}
