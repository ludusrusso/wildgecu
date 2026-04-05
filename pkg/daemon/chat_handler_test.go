package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"wildgecu/pkg/command"
	"wildgecu/pkg/provider"
	"wildgecu/pkg/session"
)

// fakeSkillCommand implements command.Command and command.SkillRunner for testing.
type fakeSkillCommand struct {
	name    string
	content string
}

func (c *fakeSkillCommand) Name() string                              { return c.name }
func (c *fakeSkillCommand) Description() string                       { return "fake skill" }
func (c *fakeSkillCommand) Execute(_ context.Context, _ string) (string, error) {
	return c.content, nil
}
func (c *fakeSkillCommand) SkillContent() string { return c.content }

// blockingProvider blocks on Generate until the context is cancelled.
type blockingProvider struct{}

func (blockingProvider) Generate(ctx context.Context, _ *provider.GenerateParams) (*provider.Response, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestInterruptDuringMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := &SocketServer{ctx: ctx}

	sm := &SessionManager{
		chatCfg: &session.Config{
			Provider:    blockingProvider{},
			WelcomeText: "hello",
		},
		sessions: make(map[string]*ManagedSession),
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := slog.Default()

	// Run handleChatConnection in the background.
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Send session.create as the first request (already decoded by dispatcher).
		firstReq := &ChatRequest{Type: "session.create"}
		srv.handleChatConnection(serverConn, firstReq, sm, logger)
	}()

	enc := json.NewEncoder(clientConn)
	dec := json.NewDecoder(clientConn)

	// Read the session.created response.
	var created ChatEvent
	if err := dec.Decode(&created); err != nil {
		t.Fatalf("decode session.created: %v", err)
	}
	if created.Type != "session.created" {
		t.Fatalf("expected session.created, got %s", created.Type)
	}
	sessionID := created.SessionID

	// Send a message — this will block inside the goroutine (provider blocks).
	enc.Encode(ChatRequest{Type: "message", SessionID: sessionID, Content: "hi"})

	// Give the goroutine a moment to start processing the message.
	time.Sleep(50 * time.Millisecond)

	// Send an interrupt — this should be processed immediately because
	// handleChatMessage runs in a goroutine and the read loop is free.
	enc.Encode(ChatRequest{Type: "session.interrupt", SessionID: sessionID})

	// The blocked provider should now be cancelled, producing an error event.
	var ev ChatEvent
	if err := dec.Decode(&ev); err != nil {
		t.Fatalf("decode after interrupt: %v", err)
	}
	if ev.Type != "error" {
		t.Fatalf("expected error event after interrupt, got %s: %+v", ev.Type, ev)
	}
}

// newTestServer creates a SocketServer with a command registry containing /help.
func newTestServer(ctx context.Context) *SocketServer {
	reg := command.NewRegistry("")
	help := command.NewHelpCommand(reg)
	reg.Register(help)
	return &SocketServer{ctx: ctx, commands: reg}
}

// newTestServerWithClean creates a SocketServer with /help and /clean commands
// backed by the given SessionManager.
func newTestServerWithClean(ctx context.Context, sm *SessionManager) *SocketServer {
	reg := command.NewRegistry("")
	help := command.NewHelpCommand(reg)
	reg.Register(help)
	clean := command.NewCleanCommand(func(cmdCtx context.Context, id string) (string, error) {
		return sm.ResetSession(cmdCtx, id)
	})
	reg.Register(clean)
	return &SocketServer{ctx: ctx, commands: reg}
}

func TestSlashCommandHelp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := newTestServer(ctx)

	sm := &SessionManager{
		chatCfg: &session.Config{
			Provider:    blockingProvider{},
			WelcomeText: "hello",
		},
		sessions: make(map[string]*ManagedSession),
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := slog.Default()

	done := make(chan struct{})
	go func() {
		defer close(done)
		firstReq := &ChatRequest{Type: "session.create"}
		srv.handleChatConnection(serverConn, firstReq, sm, logger)
	}()

	enc := json.NewEncoder(clientConn)
	dec := json.NewDecoder(clientConn)

	// Read session.created
	var created ChatEvent
	if err := dec.Decode(&created); err != nil {
		t.Fatalf("decode session.created: %v", err)
	}

	// Send /help command
	enc.Encode(ChatRequest{Type: "message", SessionID: created.SessionID, Content: "/help"})

	var ev ChatEvent
	if err := dec.Decode(&ev); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ev.Type != "done" {
		t.Fatalf("expected done event, got %s: %+v", ev.Type, ev)
	}
	if ev.Content == "" {
		t.Fatal("expected non-empty help output")
	}
	if !strings.Contains(ev.Content, "/help") {
		t.Errorf("expected help output to contain '/help', got %q", ev.Content)
	}
}

func TestSlashCommandUnknown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := newTestServer(ctx)

	sm := &SessionManager{
		chatCfg: &session.Config{
			Provider:    blockingProvider{},
			WelcomeText: "hello",
		},
		sessions: make(map[string]*ManagedSession),
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := slog.Default()

	done := make(chan struct{})
	go func() {
		defer close(done)
		firstReq := &ChatRequest{Type: "session.create"}
		srv.handleChatConnection(serverConn, firstReq, sm, logger)
	}()

	enc := json.NewEncoder(clientConn)
	dec := json.NewDecoder(clientConn)

	var created ChatEvent
	if err := dec.Decode(&created); err != nil {
		t.Fatalf("decode session.created: %v", err)
	}

	// Send unknown slash command
	enc.Encode(ChatRequest{Type: "message", SessionID: created.SessionID, Content: "/typo"})

	var ev ChatEvent
	if err := dec.Decode(&ev); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ev.Type != "done" {
		t.Fatalf("expected done event, got %s: %+v", ev.Type, ev)
	}
	if !strings.Contains(ev.Content, "Unknown command: /typo") {
		t.Errorf("expected unknown command error, got %q", ev.Content)
	}
}

func TestSlashCommandCleanResetsSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sm := newTestSessionManager(t)
	srv := newTestServerWithClean(ctx, sm)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := slog.Default()

	done := make(chan struct{})
	go func() {
		defer close(done)
		firstReq := &ChatRequest{Type: "session.create"}
		srv.handleChatConnection(serverConn, firstReq, sm, logger)
	}()

	enc := json.NewEncoder(clientConn)
	dec := json.NewDecoder(clientConn)

	// Read session.created
	var created ChatEvent
	if err := dec.Decode(&created); err != nil {
		t.Fatalf("decode session.created: %v", err)
	}
	oldSessionID := created.SessionID

	// Verify old session exists.
	if sm.Get(oldSessionID) == nil {
		t.Fatal("expected old session to exist before /clean")
	}

	// Send /clean command
	if err := enc.Encode(ChatRequest{Type: "message", SessionID: oldSessionID, Content: "/clean"}); err != nil {
		t.Fatalf("encode /clean: %v", err)
	}

	var ev ChatEvent
	if err := dec.Decode(&ev); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if ev.Type != "done" {
		t.Fatalf("expected done event, got %s: %+v", ev.Type, ev)
	}
	if !strings.Contains(ev.Content, "Session reset") {
		t.Errorf("expected reset confirmation, got %q", ev.Content)
	}
	if !strings.Contains(ev.Content, "New session:") {
		t.Errorf("expected new session info, got %q", ev.Content)
	}

	// Old session should be removed.
	if sm.Get(oldSessionID) != nil {
		t.Error("expected old session to be removed after /clean")
	}

	// Extract new session ID from response and verify it exists.
	const prefix = "New session: "
	idx := strings.Index(ev.Content, prefix)
	if idx < 0 {
		t.Fatalf("could not find new session ID in response: %q", ev.Content)
	}
	newSessionID := ev.Content[idx+len(prefix):]
	if sm.Get(newSessionID) == nil {
		t.Error("expected new session to exist after /clean")
	}
}

func TestCommandsList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := newTestServer(ctx)

	sm := &SessionManager{
		chatCfg: &session.Config{
			Provider:    blockingProvider{},
			WelcomeText: "hello",
		},
		sessions: make(map[string]*ManagedSession),
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := slog.Default()

	done := make(chan struct{})
	go func() {
		defer close(done)
		firstReq := &ChatRequest{Type: "commands.list"}
		srv.handleChatConnection(serverConn, firstReq, sm, logger)
	}()

	dec := json.NewDecoder(clientConn)

	var ev ChatEvent
	if err := dec.Decode(&ev); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ev.Type != "commands.list" {
		t.Fatalf("expected commands.list event, got %s: %+v", ev.Type, ev)
	}
	if len(ev.Commands) == 0 {
		t.Fatal("expected non-empty commands list")
	}

	// The test server has /help registered; verify it's in the list.
	found := false
	for _, cmd := range ev.Commands {
		if cmd.Name == "help" {
			found = true
			if cmd.Description == "" {
				t.Error("expected non-empty description for /help")
			}
			break
		}
	}
	if !found {
		t.Errorf("expected /help in commands list, got %+v", ev.Commands)
	}
}

func TestSlashSkillCommandDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sm := newTestSessionManager(t)

	reg := command.NewRegistry("")
	reg.Register(&fakeSkillCommand{name: "review", content: "Review the code carefully."})
	srv := &SocketServer{ctx: ctx, commands: reg}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	logger := slog.Default()

	done := make(chan struct{})
	go func() {
		defer close(done)
		firstReq := &ChatRequest{Type: "session.create"}
		srv.handleChatConnection(serverConn, firstReq, sm, logger)
	}()

	enc := json.NewEncoder(clientConn)
	dec := json.NewDecoder(clientConn)

	var created ChatEvent
	if err := dec.Decode(&created); err != nil {
		t.Fatalf("decode session.created: %v", err)
	}

	if err := enc.Encode(ChatRequest{Type: "message", SessionID: created.SessionID, Content: "/review some code"}); err != nil {
		t.Fatalf("encode: %v", err)
	}

	var ev ChatEvent
	if err := dec.Decode(&ev); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ev.Type != "done" {
		t.Fatalf("expected done event, got %s: %+v", ev.Type, ev)
	}
	if ev.Content == "" {
		t.Error("expected non-empty skill response")
	}
}
