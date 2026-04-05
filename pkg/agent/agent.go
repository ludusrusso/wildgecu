package agent

import (
	"context"
	"fmt"
	"time"

	"wildgecu/pkg/agent/tools"
	"wildgecu/pkg/home"
	"wildgecu/pkg/provider"
	"wildgecu/pkg/provider/tool"
	"wildgecu/pkg/session"
	"wildgecu/pkg/telegram/auth"
	"wildgecu/x/debug"
)

// Config holds the configuration needed to run the agent.
type Config struct {
	Provider     provider.Provider
	Home         *home.Home
	Workspace    *home.Home
	TelegramAuth *auth.Store // nil when Telegram auth is not configured
	Debug        bool
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
	if err != nil {
		return nil, dbg, fmt.Errorf("loading soul: %w", err)
	}

	if soulContent == "" {
		return nil, dbg, fmt.Errorf("SOUL.md not found; run 'wildgecu init' to bootstrap your agent")
	}

	memoryContent, memErr := LoadMemory(cfg.Home)
	if memErr != nil {
		return nil, dbg, fmt.Errorf("loading memory: %w", memErr)
	}

	skillsDir := cfg.Home.SkillsDir()
	registry := tool.NewRegistry()
	registry.Add(tools.GeneralTools())
	registry.Add(tools.ExecTools(cfg.Home.Dir()))
	registry.Add(tools.SkillTools(skillsDir))
	registry.Add(tools.InformTools())
	registry.Add(tools.TelegramTools(cfg.TelegramAuth))
	systemPrompt := BuildSystemPrompt(cfg.Workspace, soulContent, memoryContent)
	if dbg != nil {
		dbg.SystemPrompt(systemPrompt)
	}

	chatCfg := &session.Config{
		Provider:     cfg.Provider,
		SystemPrompt: systemPrompt,
		Tools:        registry.Tools(),
		Executor:     registry.Executor(),
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
	if err != nil {
		return err
	}

	memCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return RunMemoryAgent(memCtx, cfg.Provider, cfg.Home, messages, memoryContent)
}

// PrepareCode sets up a code-mode session with file tools and bash running in workDir.
func PrepareCode(ctx context.Context, cfg Config, workDir string) (*session.Config, *debug.Logger, error) {
	var dbg *debug.Logger
	if cfg.Debug {
		var err error
		dbg, err = debug.New()
		if err != nil {
			return nil, nil, fmt.Errorf("debug logger: %w", err)
		}
	}

	soulContent, err := LoadSoul(cfg.Home)
	if err != nil {
		return nil, dbg, fmt.Errorf("loading soul: %w", err)
	}
	if soulContent == "" {
		return nil, dbg, fmt.Errorf("SOUL.md not found; run 'wildgecu init' to bootstrap your agent")
	}

	memoryContent, memErr := LoadMemory(cfg.Home)
	if memErr != nil {
		return nil, dbg, fmt.Errorf("loading memory: %w", memErr)
	}

	skillsDir := cfg.Home.SkillsDir()
	registry := tool.NewRegistry()
	registry.Add(tools.GeneralTools())
	registry.Add(tools.ExecTools(workDir))
	registry.Add(tools.FileTools(workDir))
	registry.Add(tools.SkillTools(skillsDir))
	registry.Add(tools.InformTools())
	registry.Add(tools.TelegramTools(cfg.TelegramAuth))
	systemPrompt := BuildCodeSystemPrompt(cfg.Workspace, soulContent, memoryContent, workDir)
	if dbg != nil {
		dbg.SystemPrompt(systemPrompt)
	}

	codeCfg := &session.Config{
		Provider:     cfg.Provider,
		SystemPrompt: systemPrompt,
		Tools:        registry.Tools(),
		Executor:     registry.Executor(),
		WelcomeText:  "Code agent ready. Working directory: " + workDir,
		Debug:        dbg,
	}

	return codeCfg, dbg, nil
}
