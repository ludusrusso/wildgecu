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
	// Provider name: "gemini", "openai", or "ollama".
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

const defaultOllamaURL = "http://localhost:11434/v1"

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

	case "ollama":
		url := cfg.OllamaURL
		if url == "" {
			url = defaultOllamaURL
		}
		return openai.New("", cfg.Model, openai.WithBaseURL(url)), nil

	default:
		return nil, fmt.Errorf("unknown provider %q; supported: gemini, openai, ollama", cfg.Provider)
	}
}
