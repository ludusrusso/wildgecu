// Package factory creates LLM providers from configuration.
package factory

import (
	"context"
	"fmt"

	"wildgecu/pkg/provider"
	"wildgecu/pkg/provider/gemini"
	"wildgecu/pkg/provider/openai"
)

// Config holds the parameters needed to create any supported provider.
type Config struct {
	// Provider name: "gemini", "openai", "ollama", "mistral", or "regolo".
	Provider string
	// Model identifier (provider-specific, e.g. "gpt-4o", "gemini-3-flash-preview", "llama3").
	Model string
	// APIKey for the chosen provider. Not required for Ollama.
	APIKey string
	// GoogleSearch enables Gemini's built-in Google Search grounding (Gemini only).
	GoogleSearch bool
	// OllamaURL is the base URL for the Ollama OpenAI-compatible API.
	// Defaults to "http://localhost:11434/v1" when empty.
	OllamaURL string
}

// New creates a provider.Provider based on the configuration.
func New(ctx context.Context, cfg Config) (provider.Provider, error) {
	switch cfg.Provider {
	case "gemini", "":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini provider requires an API key; set gemini_api_key in config or GEMINI_API_KEY env var")
		}
		var opts []gemini.Option
		if cfg.GoogleSearch {
			opts = append(opts, gemini.WithGoogleSearch())
		}
		return gemini.New(ctx, cfg.APIKey, cfg.Model, opts...)

	case "openai":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openai provider requires an API key; set openai_api_key in config or OPENAI_API_KEY env var")
		}
		return openai.New(cfg.APIKey, cfg.Model), nil

	case "mistral":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("mistral provider requires an API key; set mistral_api_key in config or MISTRAL_API_KEY env var")
		}
		return openai.NewMistral(cfg.APIKey, cfg.Model), nil

	case "regolo":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("regolo provider requires an API key; set regolo_api_key in config or REGOLO_API_KEY env var")
		}
		return openai.NewRegolo(cfg.APIKey, cfg.Model), nil

	case "ollama":
		var opts []openai.Option
		if cfg.OllamaURL != "" {
			opts = append(opts, openai.WithBaseURL(cfg.OllamaURL))
		}
		return openai.NewOllama(cfg.Model, opts...), nil

	default:
		return nil, fmt.Errorf("unknown provider %q; supported: gemini, openai, mistral, regolo, ollama", cfg.Provider)
	}
}
