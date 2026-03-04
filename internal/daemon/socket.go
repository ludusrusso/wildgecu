package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
)

// CommandHandler processes a daemon request and returns a response.
type CommandHandler func(*Request) (*Response, error)

// SocketServer listens on a Unix domain socket and dispatches commands.
type SocketServer struct {
	path     string
	listener net.Listener
	handlers map[string]CommandHandler
	logger   *slog.Logger
	wg       sync.WaitGroup
}

// NewSocketServer creates and starts listening on the Unix socket.
func NewSocketServer(logger *slog.Logger) (*SocketServer, error) {
	path, err := sockPath()
	if err != nil {
		return nil, err
	}

	// Remove stale socket if present.
	os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}

	return &SocketServer{
		path:     path,
		listener: ln,
		handlers: make(map[string]CommandHandler),
		logger:   logger,
	}, nil
}

// Handle registers a handler for the given command name.
func (s *SocketServer) Handle(cmd string, h CommandHandler) {
	s.handlers[cmd] = h
}

// Serve accepts connections until ctx is cancelled.
func (s *SocketServer) Serve(ctx context.Context) {
	go func() {
		<-ctx.Done()
		s.listener.Close()
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
			s.handleConnection(conn)
		}()
	}
}

// Close cleans up the socket file and closes the listener.
func (s *SocketServer) Close() {
	s.listener.Close()
	s.wg.Wait()
	os.Remove(s.path)
}

func (s *SocketServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		s.logger.Error("decode request", "error", err)
		return
	}

	handler, ok := s.handlers[req.Cmd]
	if !ok {
		resp := &Response{OK: false, Error: "unknown command: " + req.Cmd}
		json.NewEncoder(conn).Encode(resp)
		return
	}

	resp, err := handler(&req)
	if err != nil {
		resp = &Response{OK: false, Error: err.Error()}
	}
	json.NewEncoder(conn).Encode(resp)
}
