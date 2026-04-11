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

// Config is the top-level application configuration.
type Config struct {
	Providers     map[string]ProviderConfig `yaml:"providers"`
	DefaultModel  string                    `yaml:"default_model"`
	TelegramToken string                    `yaml:"telegram_token"`
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

	for _, field := range []*string{&c.TelegramToken, &c.DefaultModel} {
		if resolved, envVar, isEnv := resolveEnvValue(*field); isEnv {
			if envVar != "" {
				return fmt.Errorf("config: env var %q is not set", envVar)
			}
			*field = resolved
		}
	}

	return nil
}

func (c *Config) validate() error {
	if len(c.Providers) == 0 {
		return fmt.Errorf("config: providers map must not be empty")
	}
	if c.DefaultModel == "" {
		return fmt.Errorf("config: default_model is required")
	}

	parts := strings.SplitN(c.DefaultModel, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("config: default_model must be in provider/model format, got %q", c.DefaultModel)
	}

	if _, ok := c.Providers[parts[0]]; !ok {
		return fmt.Errorf("config: default_model references unknown provider %q", parts[0])
	}

	for name, p := range c.Providers {
		if p.Type == "" {
			return fmt.Errorf("config: provider %q must have a type", name)
		}
	}

	return nil
}
