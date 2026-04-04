package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"testing"
	"time"

	"wildgecu/pkg/provider"
	"wildgecu/pkg/session"
)

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
