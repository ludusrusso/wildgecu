package daemon

import (
	"encoding/json"
	"fmt"
	"net"
)

// Event is a server→client message over the NDJSON socket.
type Event struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Welcome   string `json:"welcome,omitempty"`
	Name      string `json:"name,omitempty"`
	Args      string `json:"args,omitempty"`
	Message   string `json:"message,omitempty"`
}

// request is a client→server message over the NDJSON socket.
type request struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Mode      string `json:"mode,omitempty"`
	WorkDir   string `json:"work_dir,omitempty"`
}

// Client communicates with the daemon over a long-lived Unix socket using NDJSON.
type Client struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
}

// Connect opens a connection to the daemon socket.
func Connect(socketPath string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	return &Client{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
	}, nil
}

// CreateSession asks the daemon to create a new chat session.
// Returns the session ID and the welcome text.
func (c *Client) CreateSession() (sessionID, welcome string, err error) {
	if err := c.encoder.Encode(request{Type: "session.create"}); err != nil {
		return "", "", fmt.Errorf("send session.create: %w", err)
	}
	var ev Event
	if err := c.decoder.Decode(&ev); err != nil {
		return "", "", fmt.Errorf("read session.created: %w", err)
	}
	if ev.Type == "error" {
		return "", "", fmt.Errorf("session.create failed: %s", ev.Message)
	}
	return ev.SessionID, ev.Welcome, nil
}

// CreateCodeSession asks the daemon to create a new code-mode session.
// Returns the session ID and the welcome text.
func (c *Client) CreateCodeSession(workDir string) (sessionID, welcome string, err error) {
	if err := c.encoder.Encode(request{Type: "session.create", Mode: "code", WorkDir: workDir}); err != nil {
		return "", "", fmt.Errorf("send session.create (code): %w", err)
	}
	var ev Event
	if err := c.decoder.Decode(&ev); err != nil {
		return "", "", fmt.Errorf("read session.created: %w", err)
	}
	if ev.Type == "error" {
		return "", "", fmt.Errorf("session.create failed: %s", ev.Message)
	}
	return ev.SessionID, ev.Welcome, nil
}

// SendMessage sends a user message to the given session.
// The caller should then call ReadEvent in a loop to receive streaming events.
func (c *Client) SendMessage(sessionID, content string) error {
	return c.encoder.Encode(request{
		Type:      "message",
		SessionID: sessionID,
		Content:   content,
	})
}

// ReadEvent reads the next NDJSON event from the server.
func (c *Client) ReadEvent() (*Event, error) {
	var ev Event
	if err := c.decoder.Decode(&ev); err != nil {
		return nil, err
	}
	return &ev, nil
}


// InterruptSession asks the daemon to interrupt the current turn of the session.
func (c *Client) InterruptSession(sessionID string) error {
	return c.encoder.Encode(request{
		Type:      "session.interrupt",
		SessionID: sessionID,
	})
}

// CloseSession asks the daemon to close and finalize the session.
func (c *Client) CloseSession(sessionID string) error {
	return c.encoder.Encode(request{
		Type:      "session.close",
		SessionID: sessionID,
	})
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
