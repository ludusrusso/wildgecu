package agent

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"wildgecu/x/debug"
	"wildgecu/x/home"
	"wildgecu/pkg/provider"
	"wildgecu/pkg/provider/tool"
	"wildgecu/pkg/session"
)

// Config holds the configuration needed to run the agent.
type Config struct {
	Provider  provider.Provider
	Home      home.Home
	Workspace home.Home
	HomeDir   string // absolute path to ~/.wildgecu, used as bash tool working directory
	Debug     bool
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
	if err != nil && !errors.Is(err, home.ErrNotFound) {
		return nil, dbg, fmt.Errorf("loading soul: %w", err)
	}

	memoryContent, memErr := LoadMemory(cfg.Home)
	if memErr != nil && !errors.Is(memErr, home.ErrNotFound) {
		return nil, dbg, fmt.Errorf("loading memory: %w", memErr)
	}

	if errors.Is(err, home.ErrNotFound) {
		// Bootstrap needs to run in the old direct way.
		// For now, skip bootstrap when running under daemon.
		// The daemon requires a pre-existing soul.
		return nil, dbg, fmt.Errorf("soul not found: run 'wildgecu chat' directly to bootstrap your agent first")
	}

	skillsDir := filepath.Join(cfg.HomeDir, "skills")
	tools := loadTools(skillsDir, cfg.HomeDir)
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
	if err != nil && !errors.Is(err, home.ErrNotFound) {
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
	if err != nil && !errors.Is(err, home.ErrNotFound) {
		return nil, dbg, fmt.Errorf("loading soul: %w", err)
	}
	if errors.Is(err, home.ErrNotFound) {
		return nil, dbg, fmt.Errorf("soul not found: run 'wildgecu chat' directly to bootstrap your agent first")
	}

	memoryContent, memErr := LoadMemory(cfg.Home)
	if memErr != nil && !errors.Is(memErr, home.ErrNotFound) {
		return nil, dbg, fmt.Errorf("loading memory: %w", memErr)
	}

	skillsDir := filepath.Join(cfg.HomeDir, "skills")
	tools := loadCodeTools(skillsDir, workDir)
	systemPrompt := BuildCodeSystemPrompt(cfg.Workspace, soulContent, memoryContent, workDir)
	if dbg != nil {
		dbg.SystemPrompt(systemPrompt)
	}

	codeCfg := &session.Config{
		Provider:     cfg.Provider,
		SystemPrompt: systemPrompt,
		Tools:        tools.Tools(),
		Executor:     tools.Executor(),
		WelcomeText:  "Code agent ready. Working directory: " + workDir,
		Debug:        dbg,
	}

	return codeCfg, dbg, nil
}

func loadTools(skillsDir string, homeDir string) *tool.Registry {
	tools := []tool.Tool{getCurrentTimeTool, newBashTool(homeDir), newNodeTool(homeDir)}
	if skillsDir != "" {
		tools = append(tools, newLoadSkillTool(skillsDir))
	}
	return tool.NewRegistry(tools...)
}

func loadCodeTools(skillsDir string, workDir string) *tool.Registry {
	tools := []tool.Tool{
		getCurrentTimeTool,
		newBashTool(workDir),
		newNodeTool(workDir),
		newListFilesTool(workDir),
		newReadFileTool(workDir),
		newWriteFileTool(workDir),
		newUpdateFileTool(workDir),
	}
	if skillsDir != "" {
		tools = append(tools, newLoadSkillTool(skillsDir))
	}
	return tool.NewRegistry(tools...)
}
