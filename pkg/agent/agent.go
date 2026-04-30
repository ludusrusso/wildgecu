package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ludusrusso/wildgecu/pkg/agent/tools"
	"github.com/ludusrusso/wildgecu/pkg/home"
	"github.com/ludusrusso/wildgecu/pkg/provider"
	"github.com/ludusrusso/wildgecu/pkg/provider/tool"
	"github.com/ludusrusso/wildgecu/pkg/session"
	"github.com/ludusrusso/wildgecu/pkg/telegram/auth"
	"github.com/ludusrusso/wildgecu/x/debug"
)

// Config holds the configuration needed to run the agent.
type Config struct {
	Provider        provider.Provider
	Home            *home.Home
	Workspace       *home.Home
	TelegramAuth    *auth.Store            // nil when Telegram auth is not configured
	ResolveProvider tools.ProviderResolver // nil when model override is not supported
	MemoryModel     string                 // optional alias/ref for the memory agent; empty = use Provider
	ModelsInfo      *tools.ModelInfo       // nil when model info is not available
	Tools           tools.Config           // tool configuration; zero values pick sensible defaults
	Debug           bool
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
	registry.Add(tools.SearchTools(cfg.Home.Dir(), cfg.Tools.Search))
	registry.Add(tools.MultiEditTools(cfg.Home.Dir()))
	registry.Add(tools.SkillTools(skillsDir))
	registry.Add(tools.InformTools())
	registry.Add(tools.TelegramTools(cfg.TelegramAuth))
	registry.Add(tools.TodoTools())
	registry.Add(tools.SubagentTools(cfg.Provider, registry, cfg.ResolveProvider))
	registry.Add(tools.ModelTools(cfg.ModelsInfo))
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

	p, modelLabel := resolveMemoryProvider(memCtx, cfg)
	return RunMemoryAgent(memCtx, p, modelLabel, cfg.Home, messages, memoryContent)
}

// resolveMemoryProvider picks the provider to use for the memory agent. When
// cfg.MemoryModel is set and a resolver is available, it resolves a dedicated
// provider; otherwise it falls back to cfg.Provider. The returned label is a
// human-readable tag used only for logging.
func resolveMemoryProvider(ctx context.Context, cfg Config) (provider.Provider, string) {
	if cfg.MemoryModel != "" && cfg.ResolveProvider != nil {
		p, err := cfg.ResolveProvider(ctx, cfg.MemoryModel)
		if err == nil {
			return p, cfg.MemoryModel
		}
		slog.Warn("memory agent: resolve memory_model failed, falling back to default provider", "model", cfg.MemoryModel, "error", err)
	}
	return cfg.Provider, "default"
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
	registry.Add(tools.SearchTools(workDir, cfg.Tools.Search))
	registry.Add(tools.SkillTools(skillsDir))
	registry.Add(tools.InformTools())
	registry.Add(tools.TelegramTools(cfg.TelegramAuth))
	registry.Add(tools.TodoTools())
	registry.Add(tools.SubagentTools(cfg.Provider, registry, cfg.ResolveProvider))
	registry.Add(tools.ModelTools(cfg.ModelsInfo))
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
