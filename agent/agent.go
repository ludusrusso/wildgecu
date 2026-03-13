package agent

import (
	"context"
	"errors"
	"fmt"

	"gonesis/chat"
	"gonesis/debug"
	"gonesis/homer"
	"gonesis/provider"
	"gonesis/provider/tool"
	"gonesis/tui"
)

// Config holds the configuration needed to run the agent.
type Config struct {
	Provider   provider.Provider
	Home       homer.Homer
	Workspace  homer.Homer
	SkillsHome homer.Homer
	Debug      bool
}

// Run loads the soul (bootstrapping if needed) and starts the agent chat loop.
func Run(ctx context.Context, cfg Config) error {
	var dbg *debug.Logger
	if cfg.Debug {
		var err error
		dbg, err = debug.New()
		if err != nil {
			return fmt.Errorf("debug logger: %w", err)
		}
		defer dbg.Close()
	}

	soulContent, err := LoadSoul(cfg.Home)
	if err != nil && !errors.Is(err, homer.ErrNotFound) {
		return fmt.Errorf("loading soul: %w", err)
	}

	if errors.Is(err, homer.ErrNotFound) {
		bootstrapCfg := BootstrapConfig(ctx, cfg.Provider, cfg.Home, &soulContent)
		bootstrapCfg.Debug = dbg
		if err := tui.Run(ctx, bootstrapCfg); err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}
		if soulContent == "" {
			return fmt.Errorf("bootstrap did not produce a soul")
		}
	}

	tools := []tool.Tool{getCurrentTimeTool}
	if cfg.SkillsHome != nil {
		tools = append(tools, newLoadSkillTool(cfg.SkillsHome))
	}
	registry := tool.NewRegistry(tools...)

	systemPrompt := BuildSystemPrompt(cfg.Workspace, soulContent)
	dbg.SystemPrompt(systemPrompt)

	chatCfg := &chat.Config{
		Provider:     cfg.Provider,
		SystemPrompt: systemPrompt,
		Tools:        registry.Tools(),
		Executor:     registry.Executor(),
		WelcomeText:  "Agent ready.",
		Debug:        dbg,
	}
	return tui.Run(ctx, chatCfg)
}
