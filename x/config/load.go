package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

var envPattern = regexp.MustCompile(`^env\(([^)]+)\)$`)

// ProviderConfig holds configuration for a single LLM provider.
type ProviderConfig struct {
	Type         string `yaml:"type"`
	APIKey       string `yaml:"api_key"`
	BaseURL      string `yaml:"base_url"`
	GoogleSearch bool   `yaml:"google_search"`
}

// GrepConfig configures the grep tool. Zero values fall through to the
// defaults baked into pkg/search.
type GrepConfig struct {
	MaxResults       int   `yaml:"max_results"`
	MaxFileSizeBytes int64 `yaml:"max_file_size_bytes"`
}

// BashConfig configures the bash tool. Zero values fall through to the
// defaults baked into pkg/exec/bounded.
type BashConfig struct {
	MaxTimeoutSeconds int `yaml:"max_timeout_seconds"`
	HeadBytes         int `yaml:"head_bytes"`
	TailBytes         int `yaml:"tail_bytes"`
}

// ToolsConfig groups per-tool configuration loaded from the YAML "tools" block.
type ToolsConfig struct {
	Grep GrepConfig `yaml:"grep"`
	Bash BashConfig `yaml:"bash"`
}

// Config is the top-level application configuration.
type Config struct {
	Providers     map[string]ProviderConfig `yaml:"providers"`
	Models        map[string]string         `yaml:"models"`
	DefaultModel  string                    `yaml:"default_model"`
	MemoryModel   string                    `yaml:"memory_model"`
	TelegramToken string                    `yaml:"telegram_token"`
	Tools         ToolsConfig               `yaml:"tools"`
}

// Load reads and validates a YAML config file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.resolveEnv(); err != nil {
		return nil, err
	}

	cfg.applyProviderDefaults()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func resolveEnvValue(val string) (string, string, bool) {
	m := envPattern.FindStringSubmatch(val)
	if m == nil {
		return val, "", false
	}
	envVar := m[1]
	resolved, ok := os.LookupEnv(envVar)
	if !ok {
		return "", envVar, true
	}
	return resolved, "", true
}

func (c *Config) resolveEnv() error {
	for name, p := range c.Providers {
		for _, field := range []*string{&p.APIKey, &p.BaseURL} {
			if resolved, envVar, isEnv := resolveEnvValue(*field); isEnv {
				if envVar != "" {
					return fmt.Errorf("config: provider %q: env var %q is not set", name, envVar)
				}
				*field = resolved
			}
		}
		c.Providers[name] = p
	}

	for _, field := range []*string{&c.TelegramToken, &c.DefaultModel, &c.MemoryModel} {
		if resolved, envVar, isEnv := resolveEnvValue(*field); isEnv {
			if envVar != "" {
				return fmt.Errorf("config: env var %q is not set", envVar)
			}
			*field = resolved
		}
	}

	return nil
}

// KnownBaseURLs maps provider types to their default base URLs.
var KnownBaseURLs = map[string]string{
	"mistral": "https://api.mistral.ai/v1",
	"regolo":  "https://api.regolo.ai/v1",
	"ollama":  "http://localhost:11434/v1",
}

func (c *Config) applyProviderDefaults() {
	for name, p := range c.Providers {
		if p.BaseURL == "" {
			if defaultURL, ok := KnownBaseURLs[p.Type]; ok {
				p.BaseURL = defaultURL
				c.Providers[name] = p
			}
		}
	}
}

func (c *Config) validate() error {
	if len(c.Providers) == 0 {
		return fmt.Errorf("config: providers map must not be empty")
	}

	for name, target := range c.Models {
		if strings.Contains(name, "/") {
			return fmt.Errorf("config: model alias %q must not contain '/'", name)
		}
		if err := c.validateProviderModel(target); err != nil {
			return fmt.Errorf("config: model alias %q: %w", name, err)
		}
	}

	if c.DefaultModel == "" {
		return fmt.Errorf("config: default_model is required")
	}

	if err := c.ValidateModelRef(c.DefaultModel); err != nil {
		return fmt.Errorf("config: default_model: %w", err)
	}

	if c.MemoryModel != "" {
		if err := c.ValidateModelRef(c.MemoryModel); err != nil {
			return fmt.Errorf("config: memory_model: %w", err)
		}
	}

	for name, p := range c.Providers {
		if p.Type == "" {
			return fmt.Errorf("config: provider %q must have a type", name)
		}
	}

	return nil
}

// validateProviderModel checks that s is a valid "provider/model" string
// referencing a known provider.
func (c *Config) validateProviderModel(s string) error {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("must be in provider/model format, got %q", s)
	}
	if _, ok := c.Providers[parts[0]]; !ok {
		return fmt.Errorf("references unknown provider %q", parts[0])
	}
	return nil
}

// ValidateModelRef checks that s is either a valid "provider/model" string or
// a known alias from the Models map.
func (c *Config) ValidateModelRef(s string) error {
	if strings.Contains(s, "/") {
		return c.validateProviderModel(s)
	}
	if _, ok := c.Models[s]; !ok {
		return fmt.Errorf("unknown model alias %q", s)
	}
	return nil
}
