package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gonesis/chat"
	"gonesis/provider"
	"gonesis/tui"
)

// Config holds the configuration needed to run the agent.
type Config struct {
	Provider provider.Provider
	BaseDir  string
}

// Run loads the soul (bootstrapping if needed) and starts the agent chat loop.
func Run(ctx context.Context, cfg Config) error {
	soulContent, exists, err := LoadSoul(cfg.BaseDir)
	if err != nil {
		return fmt.Errorf("loading soul: %w", err)
	}

	if !exists {
		bootstrapCfg := BootstrapConfig(ctx, cfg.Provider, cfg.BaseDir, &soulContent)
		if err := tui.Run(ctx, bootstrapCfg); err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}
		if soulContent == "" {
			return fmt.Errorf("bootstrap did not produce a soul")
		}
	}

	tools := []provider.Tool{}

	systemPrompt := BuildSystemPrompt(cfg.BaseDir, soulContent)
	chatCfg := &chat.Config{
		Provider:     cfg.Provider,
		SystemPrompt: systemPrompt,
		Tools:        tools,
		Executor:     func(tc provider.ToolCall) (string, error) { return executeTool(tc), nil },
		WelcomeText:  "Agent ready.",
	}
	return tui.Run(ctx, chatCfg)
}

func executeTool(tc provider.ToolCall) string {
	switch tc.Name {
	case "get_current_time":
		tz := "UTC"
		if v, ok := tc.Args["timezone"].(string); ok {
			tz = v
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return fmt.Sprintf(`{"error": %q}`, err.Error())
		}
		now := time.Now().In(loc)
		b, _ := json.Marshal(map[string]string{
			"time":     now.Format(time.RFC3339),
			"timezone": tz,
		})
		return string(b)
	default:
		log.Printf("unknown tool: %s", tc.Name)
		return `{"error": "unknown tool"}`
	}
}
