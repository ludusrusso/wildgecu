package daemon

import (
	"encoding/json"
	"log/slog"
	"net"
	"sync"
)

// ChatRequest is a client→server message on a NDJSON chat connection.
type ChatRequest struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

// ChatEvent is a server→client message on a NDJSON chat connection.
type ChatEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Welcome   string `json:"welcome,omitempty"`
	Name      string `json:"name,omitempty"`
	Args      string `json:"args,omitempty"`
	Message   string `json:"message,omitempty"`
}

// handleChatConnection processes a long-lived NDJSON chat connection.
// It reads requests in a loop and dispatches to the session manager.
func (s *SocketServer) handleChatConnection(conn net.Conn, firstReq *ChatRequest, sessions *SessionManager, logger *slog.Logger) {
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var encoderMu sync.Mutex
	send := func(ev ChatEvent) {
		encoderMu.Lock()
		defer encoderMu.Unlock()
		encoder.Encode(ev)
	}

	// Process the first request (already decoded by the dispatcher).
	s.dispatchChatRequest(firstReq, send, sessions, logger)

	// Continue reading subsequent requests on the same connection.
	for {
		var req ChatRequest
		if err := decoder.Decode(&req); err != nil {
			return // connection closed or error
		}
		s.dispatchChatRequest(&req, send, sessions, logger)
	}
}

func (s *SocketServer) dispatchChatRequest(req *ChatRequest, send func(ChatEvent), sessions *SessionManager, logger *slog.Logger) {
	switch req.Type {
	case "session.create":
		sess := sessions.Create()
		logger.Info("session created", "session_id", sess.ID)
		send(ChatEvent{
			Type:      "session.created",
			SessionID: sess.ID,
			Welcome:   sessions.WelcomeText(),
		})

	case "session.resume":
		sess := sessions.Get(req.SessionID)
		if sess == nil {
			send(ChatEvent{Type: "error", Message: "session not found: " + req.SessionID})
			return
		}
		send(ChatEvent{
			Type:      "session.created",
			SessionID: sess.ID,
			Welcome:   sessions.WelcomeText(),
		})

	case "message":
		go s.handleChatMessage(req, send, sessions, logger)

	case "session.interrupt":
		logger.Info("session interrupted", "session_id", req.SessionID)
		sessions.Interrupt(req.SessionID)

	case "session.close":
		logger.Info("session closed", "session_id", req.SessionID)
		sessions.Close(s.ctx, req.SessionID)
	}
}

func (s *SocketServer) handleChatMessage(req *ChatRequest, send func(ChatEvent), sessions *SessionManager, logger *slog.Logger) {
	onChunk := func(chunk string) {
		send(ChatEvent{Type: "chunk", Content: chunk})
	}
	onToolCall := func(name string, args string) {
		send(ChatEvent{Type: "tool_call", Name: name, Args: args})
	}

	content, err := sessions.RunTurnStream(s.ctx, req.SessionID, req.Content, onChunk, onToolCall)
	if err != nil {
		logger.Error("chat turn error", "session_id", req.SessionID, "error", err)
		send(ChatEvent{Type: "error", Message: err.Error()})
		return
	}
	send(ChatEvent{Type: "done", Content: content})
}
