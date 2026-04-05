package daemon

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	"wildgecu/pkg/command"
)

// ChatRequest is a client→server message on a NDJSON chat connection.
type ChatRequest struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Mode      string `json:"mode,omitempty"`
	WorkDir   string `json:"work_dir,omitempty"`
}

// CommandInfo is a name+description pair returned by the commands.list event.
type CommandInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ChatEvent is a server→client message on a NDJSON chat connection.
type ChatEvent struct {
	Type      string        `json:"type"`
	SessionID string        `json:"session_id,omitempty"`
	Content   string        `json:"content,omitempty"`
	Welcome   string        `json:"welcome,omitempty"`
	Name      string        `json:"name,omitempty"`
	Args      string        `json:"args,omitempty"`
	Message   string        `json:"message,omitempty"`
	Commands  []CommandInfo `json:"commands,omitempty"`
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
		if err := encoder.Encode(ev); err != nil {
			logger.Error("encode chat event error", "error", err)
		}
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
		var sess *ManagedSession
		var welcome string
		if req.Mode == "code" {
			var err error
			sess, err = sessions.CreateCode(req.WorkDir)
			if err != nil {
				send(ChatEvent{Type: "error", Message: "create code session: " + err.Error()})
				return
			}
			welcome = sess.welcomeText
		} else {
			sess = sessions.Create()
			welcome = sessions.WelcomeText()
		}
		logger.Info("session created", "session_id", sess.ID, "mode", req.Mode)
		send(ChatEvent{
			Type:      "session.created",
			SessionID: sess.ID,
			Welcome:   welcome,
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
		if s.commands != nil && strings.HasPrefix(req.Content, "/") {
			go s.handleSlashCommand(req, send, sessions, logger)
			return
		}
		go s.handleChatMessage(req, send, sessions, logger)

	case "session.interrupt":
		logger.Info("session interrupted", "session_id", req.SessionID)
		sessions.Interrupt(req.SessionID)

	case "session.close":
		logger.Info("session closed", "session_id", req.SessionID)
		sessions.Close(s.ctx, req.SessionID)

	case "commands.list":
		var cmds []CommandInfo
		if s.commands != nil {
			for _, e := range s.commands.List() {
				cmds = append(cmds, CommandInfo{Name: e.Name, Description: e.Description})
			}
		}
		send(ChatEvent{Type: "commands.list", Commands: cmds})
	}
}

func (s *SocketServer) handleSlashCommand(req *ChatRequest, send func(ChatEvent), sessions *SessionManager, logger *slog.Logger) {
	name, args := command.Parse(req.Content)
	if name == "" {
		send(ChatEvent{Type: "done", Content: "Usage: /<command> [args]"})
		return
	}

	cmd := s.commands.Resolve(name)
	if cmd == nil {
		send(ChatEvent{Type: "done", Content: fmt.Sprintf("Unknown command: /%s", name)})
		return
	}

	// Skill commands run a streaming LLM turn with skill content as system context.
	if runner, ok := cmd.(command.SkillRunner); ok {
		s.handleSkillCommand(req, runner, args, send, sessions, logger)
		return
	}

	ctx := command.WithSessionID(s.ctx, req.SessionID)
	result, err := cmd.Execute(ctx, args)
	if err != nil {
		logger.Error("slash command error", "command", name, "error", err)
		send(ChatEvent{Type: "error", Message: err.Error()})
		return
	}
	send(ChatEvent{Type: "done", Content: result})
}

func (s *SocketServer) handleSkillCommand(req *ChatRequest, runner command.SkillRunner, userInput string, send func(ChatEvent), sessions *SessionManager, logger *slog.Logger) {
	onChunk := func(chunk string) {
		send(ChatEvent{Type: "chunk", Content: chunk})
	}
	onToolCall := func(name string, args string) {
		send(ChatEvent{Type: "tool_call", Name: name, Args: args})
	}
	onInform := func(message string) {
		send(ChatEvent{Type: "inform", Content: message})
	}

	content, err := sessions.RunSkillTurnStream(s.ctx, req.SessionID, runner.SkillContent(), userInput, onChunk, onToolCall, onInform)
	if err != nil {
		logger.Error("skill command error", "error", err)
		send(ChatEvent{Type: "error", Message: err.Error()})
		return
	}
	send(ChatEvent{Type: "done", Content: content})
}

func (s *SocketServer) handleChatMessage(req *ChatRequest, send func(ChatEvent), sessions *SessionManager, logger *slog.Logger) {
	onChunk := func(chunk string) {
		send(ChatEvent{Type: "chunk", Content: chunk})
	}
	onToolCall := func(name string, args string) {
		send(ChatEvent{Type: "tool_call", Name: name, Args: args})
	}
	onInform := func(message string) {
		send(ChatEvent{Type: "inform", Content: message})
	}

	content, err := sessions.RunTurnStream(s.ctx, req.SessionID, req.Content, onChunk, onToolCall, onInform)
	if err != nil {
		logger.Error("chat turn error", "session_id", req.SessionID, "error", err)
		send(ChatEvent{Type: "error", Message: err.Error()})
		return
	}
	send(ChatEvent{Type: "done", Content: content})
}
