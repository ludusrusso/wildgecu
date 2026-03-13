package debug

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger writes debug information to a markdown file in append mode.
type Logger struct {
	mu   sync.Mutex
	file *os.File
}

// New creates a new debug logger. It creates a file at ~/debug/<timestamp>.md.
func New() (*Logger, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("debug: get home dir: %w", err)
	}

	dir := filepath.Join(home, ".gonesis", "debug")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("debug: create dir: %w", err)
	}

	ts := time.Now().Format("2006-01-02T15-04-05")
	path := filepath.Join(dir, ts+".md")

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("debug: open file: %w", err)
	}

	l := &Logger{file: f}
	l.write("# Debug Log — %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(os.Stderr, "Debug log: %s\n", path)
	return l, nil
}

// Close closes the underlying file.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	return l.file.Close()
}

func (l *Logger) write(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.file, format, args...)
}

// SystemPrompt logs the system prompt.
func (l *Logger) SystemPrompt(prompt string) {
	if l == nil {
		return
	}
	l.write("## System Prompt\n\n```\n%s\n```\n\n", prompt)
}

// UserMessage logs a user message.
func (l *Logger) UserMessage(content string) {
	if l == nil {
		return
	}
	l.write("## User\n\n%s\n\n", content)
}

// ModelResponse logs a model response (text content).
func (l *Logger) ModelResponse(content string) {
	if l == nil {
		return
	}
	l.write("## Model\n\n%s\n\n", content)
}

// ToolCall logs a tool call from the model.
func (l *Logger) ToolCall(name string, args map[string]any) {
	if l == nil {
		return
	}
	argsJSON, _ := json.MarshalIndent(args, "", "  ")
	l.write("### Tool Call: `%s`\n\n```json\n%s\n```\n\n", name, argsJSON)
}

// ToolResult logs a tool execution result.
func (l *Logger) ToolResult(name string, result string) {
	if l == nil {
		return
	}
	l.write("### Tool Result: `%s`\n\n```\n%s\n```\n\n", name, result)
}

// GenerateRequest logs when a generate request is made to the provider.
func (l *Logger) GenerateRequest(msgCount int, toolCount int) {
	if l == nil {
		return
	}
	l.write("---\n\n> Generate request: %d messages, %d tools\n\n", msgCount, toolCount)
}

// Usage logs token usage.
func (l *Logger) Usage(input, output int) {
	if l == nil {
		return
	}
	l.write("> Usage: %d input tokens, %d output tokens\n\n", input, output)
}

// Error logs an error.
func (l *Logger) Error(err error) {
	if l == nil {
		return
	}
	l.write("### Error\n\n```\n%s\n```\n\n", err.Error())
}
