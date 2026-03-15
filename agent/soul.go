package agent

import (
	"errors"
	"fmt"
	"strings"

	"wildgecu/homer"
)

// LoadSoul reads SOUL.md from the home Homer. Returns (content, err).
// Returns homer.ErrNotFound when the file does not exist.
func LoadSoul(h homer.Homer) (string, error) {
	data, err := h.Get("SOUL.md")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeSoul writes SOUL.md to the home Homer.
func writeSoul(h homer.Homer, content string) error {
	if err := h.Upsert("SOUL.md", []byte(content)); err != nil {
		return fmt.Errorf("writing SOUL.md: %w", err)
	}
	return nil
}

// loadWorkspaceFile reads a file from the workspace Homer.
// Returns "" if the file does not exist.
func loadWorkspaceFile(ws homer.Homer, filename string) (string, error) {
	if ws == nil {
		return "", nil
	}
	data, err := ws.Get(filename)
	if errors.Is(err, homer.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", filename, err)
	}
	return string(data), nil
}

// LoadMemory reads MEMORY.md from the home Homer. Returns (content, err).
// Returns homer.ErrNotFound when the file does not exist.
func LoadMemory(h homer.Homer) (string, error) {
	data, err := h.Get("MEMORY.md")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// BuildSystemPrompt assembles the full system prompt from the embedded agent
// prompt, the runtime soul content, memory, and an optional USER.md file.
func BuildSystemPrompt(workspace homer.Homer, soulContent, memoryContent string) string {
	sections := []string{
		fmt.Sprintf("# Agent\n\n%s", strings.TrimSpace(agentPrompt)),
	}

	if s := strings.TrimSpace(soulContent); s != "" {
		sections = append(sections, fmt.Sprintf("# Agent Soul\n\n%s", s))
	}

	if m := strings.TrimSpace(memoryContent); m != "" {
		sections = append(sections, fmt.Sprintf("# Memory\n\n%s", m))
	}

	if userPrefs, err := loadWorkspaceFile(workspace, "USER.md"); err == nil && strings.TrimSpace(userPrefs) != "" {
		sections = append(sections, fmt.Sprintf("# User Preferences\n\n%s", strings.TrimSpace(userPrefs)))
	}

	return strings.Join(sections, "\n\n")
}
