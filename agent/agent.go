package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

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

	memoryContent, memErr := LoadMemory(cfg.Home)
	if memErr != nil && !errors.Is(memErr, homer.ErrNotFound) {
		return fmt.Errorf("loading memory: %w", memErr)
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

	tools := loadTools(cfg.SkillsHome)

	systemPrompt := BuildSystemPrompt(cfg.Workspace, soulContent, memoryContent)
	dbg.SystemPrompt(systemPrompt)

	var finalMessages []provider.Message
	chatCfg := &chat.Config{
		Provider:     cfg.Provider,
		SystemPrompt: systemPrompt,
		Tools:        tools.Tools(),
		Executor:     tools.Executor(),
		WelcomeText:  "Agent ready.",
		Debug:        dbg,
		OnDone: func(messages []provider.Message) {
			finalMessages = messages
		},
	}
	tuiErr := tui.Run(ctx, chatCfg)

	if len(finalMessages) > 0 {
		memCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fmt.Println("Updating memory...")
		_ = RunMemoryAgent(memCtx, cfg.Provider, cfg.Home, finalMessages, memoryContent)
	}

	return tuiErr
}

func loadTools(home homer.Homer) *tool.Registry {
	tools := []tool.Tool{getCurrentTimeTool, bashTool}
	if home != nil {
		tools = append(tools, newLoadSkillTool(home))
	}
	return tool.NewRegistry(tools...)
}
