package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gonesis/chat"
	"gonesis/homer"
	"gonesis/provider"
	"gonesis/provider/tool"
	"gonesis/tui"
)

// Config holds the configuration needed to run the agent.
type Config struct {
	Provider  provider.Provider
	Home      homer.Homer
	Workspace homer.Homer
}

// GetTimeInput is the input for the get_current_time tool.
type GetTimeInput struct {
	Timezone string `json:"timezone,omitempty" description:"IANA timezone name"`
}

// GetTimeOutput is the output for the get_current_time tool.
type GetTimeOutput struct {
	Time     string `json:"time"`
	Timezone string `json:"timezone"`
}

var getCurrentTimeTool = tool.NewTool("get_current_time", "Get the current time in a given timezone",
	func(ctx context.Context, in GetTimeInput) (GetTimeOutput, error) {
		tz := in.Timezone
		if tz == "" {
			tz = "UTC"
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return GetTimeOutput{}, fmt.Errorf("%w", err)
		}
		now := time.Now().In(loc)
		return GetTimeOutput{
			Time:     now.Format(time.RFC3339),
			Timezone: tz,
		}, nil
	},
)

// Run loads the soul (bootstrapping if needed) and starts the agent chat loop.
func Run(ctx context.Context, cfg Config) error {
	soulContent, err := LoadSoul(cfg.Home)
	if err != nil && !errors.Is(err, homer.ErrNotFound) {
		return fmt.Errorf("loading soul: %w", err)
	}

	if errors.Is(err, homer.ErrNotFound) {
		bootstrapCfg := BootstrapConfig(ctx, cfg.Provider, cfg.Home, &soulContent)
		if err := tui.Run(ctx, bootstrapCfg); err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}
		if soulContent == "" {
			return fmt.Errorf("bootstrap did not produce a soul")
		}
	}

	registry := tool.NewRegistry(getCurrentTimeTool)

	systemPrompt := BuildSystemPrompt(cfg.Workspace, soulContent)
	chatCfg := &chat.Config{
		Provider:     cfg.Provider,
		SystemPrompt: systemPrompt,
		Tools:        registry.Tools(),
		Executor:     registry.Executor(),
		WelcomeText:  "Agent ready.",
	}
	return tui.Run(ctx, chatCfg)
}
