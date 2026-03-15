package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"wildgecu/debug"
	"wildgecu/homer"
	"wildgecu/provider"
	"wildgecu/provider/tool"
	"wildgecu/session"
)

// Config holds the configuration needed to run the agent.
type Config struct {
	Provider   provider.Provider
	Home       homer.Homer
	Workspace  homer.Homer
	SkillsHome homer.Homer
	HomeDir    string // absolute path to ~/.wildgecu, used as bash tool working directory
	Debug      bool
}

// Prepare setup the agent environment, loads soul/memory and returns a session configuration.
func Prepare(ctx context.Context, cfg Config) (*session.Config, *debug.Logger, error) {
	var dbg *debug.Logger
	if cfg.Debug {
		var err error
		dbg, err = debug.New()
		if err != nil {
			return nil, nil, fmt.Errorf("debug logger: %w", err)
		}
	}

	soulContent, err := LoadSoul(cfg.Home)
	if err != nil && !errors.Is(err, homer.ErrNotFound) {
		return nil, dbg, fmt.Errorf("loading soul: %w", err)
	}

	memoryContent, memErr := LoadMemory(cfg.Home)
	if memErr != nil && !errors.Is(memErr, homer.ErrNotFound) {
		return nil, dbg, fmt.Errorf("loading memory: %w", memErr)
	}

	if errors.Is(err, homer.ErrNotFound) {
		// Bootstrap needs to run in the old direct way.
		// For now, skip bootstrap when running under daemon.
		// The daemon requires a pre-existing soul.
		return nil, dbg, fmt.Errorf("soul not found: run 'wildgecu chat' directly to bootstrap your agent first")
	}

	tools := loadTools(cfg.SkillsHome, cfg.HomeDir)
	systemPrompt := BuildSystemPrompt(cfg.Workspace, soulContent, memoryContent)
	if dbg != nil {
		dbg.SystemPrompt(systemPrompt)
	}

	chatCfg := &session.Config{
		Provider:     cfg.Provider,
		SystemPrompt: systemPrompt,
		Tools:        tools.Tools(),
		Executor:     tools.Executor(),
		WelcomeText:  "Agent ready.",
		Debug:        dbg,
	}

	return chatCfg, dbg, nil
}

// Finalize updates the agent's memory based on the conversation history.
func Finalize(ctx context.Context, cfg Config, messages []provider.Message) error {
	if len(messages) == 0 {
		return nil
	}

	memoryContent, err := LoadMemory(cfg.Home)
	if err != nil && !errors.Is(err, homer.ErrNotFound) {
		return err
	}

	memCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return RunMemoryAgent(memCtx, cfg.Provider, cfg.Home, messages, memoryContent)
}

func loadTools(home homer.Homer, homeDir string) *tool.Registry {
	tools := []tool.Tool{getCurrentTimeTool, newBashTool(homeDir), newNodeTool(homeDir)}
	if home != nil {
		tools = append(tools, newLoadSkillTool(home))
	}
	return tool.NewRegistry(tools...)
}
