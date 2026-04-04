package agent

import (
	"fmt"
	"strings"

	"wildgecu/pkg/home"
)

// LoadSoul reads SOUL.md from the home directory. Returns (content, err).
// Returns ("", nil) when the file does not exist.
func LoadSoul(h *home.Home) (string, error) {
	content, err := h.Soul().Get()
	if err != nil {
		return "", fmt.Errorf("reading SOUL.md: %w", err)
	}
	return content, nil
}

// writeSoul writes SOUL.md to the home directory.
func writeSoul(h *home.Home, content string) error {
	if err := h.Soul().Write(content); err != nil {
		return fmt.Errorf("writing SOUL.md: %w", err)
	}
	return nil
}

// loadWorkspaceFile reads USER.md from the workspace.
// Returns "" if ws is nil or the file does not exist.
func loadWorkspaceFile(ws *home.Home) (string, error) {
	if ws == nil {
		return "", nil
	}
	content, err := ws.User().Get()
	if err != nil {
		return "", fmt.Errorf("reading USER.md: %w", err)
	}
	return content, nil
}

// LoadMemory reads MEMORY.md from the home directory. Returns (content, err).
// Returns ("", nil) when the file does not exist.
func LoadMemory(h *home.Home) (string, error) {
	content, err := h.Memory().Get()
	if err != nil {
		return "", fmt.Errorf("reading MEMORY.md: %w", err)
	}
	return content, nil
}

// BuildSystemPrompt assembles the full system prompt from the embedded agent
// prompt, the runtime soul content, memory, and an optional USER.md file.
func BuildSystemPrompt(workspace *home.Home, soulContent, memoryContent string) string {
	sections := []string{
		fmt.Sprintf("# Agent\n\n%s", strings.TrimSpace(agentPrompt)),
	}

	if s := strings.TrimSpace(soulContent); s != "" {
		sections = append(sections, fmt.Sprintf("# Agent Soul\n\n%s", s))
	}

	if m := strings.TrimSpace(memoryContent); m != "" {
		sections = append(sections, fmt.Sprintf("# Memory\n\n%s", m))
	}

	if userPrefs, err := loadWorkspaceFile(workspace); err == nil && strings.TrimSpace(userPrefs) != "" {
		sections = append(sections, fmt.Sprintf("# User Preferences\n\n%s", strings.TrimSpace(userPrefs)))
	}

	return strings.Join(sections, "\n\n")
}

// BuildCodeSystemPrompt assembles the system prompt for code mode.
// It uses CODE_AGENT.md instead of AGENT.md, with {CWD} replaced by workDir.
func BuildCodeSystemPrompt(workspace *home.Home, soulContent, memoryContent, workDir string) string {
	prompt := strings.ReplaceAll(codeAgentPrompt, "{CWD}", workDir)

	sections := []string{
		fmt.Sprintf("# Agent\n\n%s", strings.TrimSpace(prompt)),
	}

	if s := strings.TrimSpace(soulContent); s != "" {
		sections = append(sections, fmt.Sprintf("# Agent Soul\n\n%s", s))
	}

	if m := strings.TrimSpace(memoryContent); m != "" {
		sections = append(sections, fmt.Sprintf("# Memory\n\n%s", m))
	}

	if userPrefs, err := loadWorkspaceFile(workspace); err == nil && strings.TrimSpace(userPrefs) != "" {
		sections = append(sections, fmt.Sprintf("# User Preferences\n\n%s", strings.TrimSpace(userPrefs)))
	}

	return strings.Join(sections, "\n\n")
}
