package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
)

// CommandHandler processes a daemon request and returns a response.
type CommandHandler func(*Request) (*Response, error)

// SocketServer listens on a Unix domain socket and dispatches commands.
type SocketServer struct {
	listener net.Listener
	handlers map[string]CommandHandler
	sessions *SessionManager
	logger   *slog.Logger
	path     string
	ctx      context.Context
	wg       sync.WaitGroup
}

// NewSocketServer creates and starts listening on the Unix socket.
func NewSocketServer(logger *slog.Logger) (*SocketServer, error) {
	path, err := sockPath()
	if err != nil {
		return nil, err
	}

	// Remove stale socket if present.
	_ = os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}

	return &SocketServer{
		path:     path,
		listener: ln,
		handlers: make(map[string]CommandHandler),
		logger:   logger,
	}, nil
}

// SetSessions configures the session manager for NDJSON chat connections.
func (s *SocketServer) SetSessions(sm *SessionManager) {
	s.sessions = sm
}

// Handle registers a handler for the given command name.
func (s *SocketServer) Handle(cmd string, h CommandHandler) {
	s.handlers[cmd] = h
}

// Serve accepts connections until ctx is cancelled.
func (s *SocketServer) Serve(ctx context.Context) {
	s.ctx = ctx

	go func() {
		<-ctx.Done()
		_ = s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			s.logger.Error("accept error", "error", err)
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(ctx, conn)
		}()
	}
}

// Close cleans up the socket file and closes the listener.
func (s *SocketServer) Close() {
	_ = s.listener.Close()
	s.wg.Wait()
	_ = os.Remove(s.path)
}

// handleConnection reads the first JSON message to determine the protocol:
// - If it has a "type" field starting with "session.", it's a NDJSON chat connection.
// - Otherwise, it's a legacy command request.
func (s *SocketServer) handleConnection(_ context.Context, conn net.Conn) {
	defer conn.Close()

	// Peek at the first JSON object to decide the protocol.
	decoder := json.NewDecoder(conn)

	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		s.logger.Error("decode first message", "error", err)
		return
	}

	// Try to detect NDJSON chat protocol by looking for "type" field.
	var probe struct {
		Type string `json:"type"`
		Cmd  string `json:"cmd"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		s.logger.Error("unmarshal probe", "error", err)
		return
	}

	if strings.HasPrefix(probe.Type, "session.") || probe.Type == "message" {
		// NDJSON chat protocol.
		if s.sessions == nil {
			encoder := json.NewEncoder(conn)
			if err := encoder.Encode(ChatEvent{Type: "error", Message: "session manager not available"}); err != nil {
				s.logger.Error("encode error event", "error", err)
			}
			return
		}
		var chatReq ChatRequest
		if err := json.Unmarshal(raw, &chatReq); err != nil {
			s.logger.Error("unmarshal chat request", "error", err)
			return
		}
		// handleChatConnection takes over the connection (long-lived).
		// We create a new decoder from the conn since the old one already consumed the buffered data.
		s.handleChatConnection(conn, &chatReq, s.sessions, s.logger)
		return
	}

	// Legacy command protocol.
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		s.logger.Error("unmarshal legacy request", "error", err)
		return
	}

	handler, ok := s.handlers[req.Cmd]
	if !ok {
		resp := &Response{OK: false, Error: "unknown command: " + req.Cmd}
		if err := json.NewEncoder(conn).Encode(resp); err != nil {
			s.logger.Error("encode response error", "error", err)
		}
		return
	}

	resp, err := handler(&req)
	if err != nil {
		resp = &Response{OK: false, Error: err.Error()}
	}
	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		s.logger.Error("encode response error", "error", err)
	}
}
