package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"gonesis/x/config"
)

// Request is a command sent from the CLI to the daemon.
type Request struct {
	Cmd  string         `json:"cmd"`
	Args map[string]any `json:"args,omitempty"`
}

// Response is the daemon's reply to a command.
type Response struct {
	OK      bool   `json:"ok"`
	Payload any    `json:"payload,omitempty"`
	Error   string `json:"error,omitempty"`
}

// sockPath returns the path to the Unix domain socket.
func sockPath() (string, error) {
	return config.GlobalFilePath("gonesis.sock")
}

// SendCommand sends a command to the running daemon and returns the response.
func SendCommand(cmd string, args map[string]any) (*Response, error) {
	path, err := sockPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("daemon not running (socket not found)")
	}

	conn, err := net.DialTimeout("unix", path, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(&Request{Cmd: cmd, Args: args}); err != nil {
		return nil, fmt.Errorf("send command: %w", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return &resp, nil
}
