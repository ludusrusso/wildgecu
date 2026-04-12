package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/ludusrusso/wildgecu/x/config"
	"go.yaml.in/yaml/v3"
)

// Validator checks provider connectivity and credentials.
// It receives the provider type, API key, and base URL.
type Validator func(providerType, apiKey, baseURL string) error

// Option configures the setup flow.
type Option func(*options)

type options struct {
	validate Validator
}

// WithValidator sets a custom provider validator.
func WithValidator(v Validator) Option {
	return func(o *options) { o.validate = v }
}

// provider describes a configurable LLM provider.
type provider struct {
	Name      string
	Type      string
	Models    []string
	BaseURL   string // default base URL; empty if not applicable
	APIKeyEnv string // env var name for the API key; empty if no key required
	Supported bool   // true if fully implemented in the setup wizard
}

var providers = []provider{
	{Name: "Gemini", Type: "gemini", APIKeyEnv: "GEMINI_API_KEY", Supported: true, Models: []string{"gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.0-flash"}},                    //nolint:gosec // env var name, not a credential
	{Name: "OpenAI", Type: "openai", APIKeyEnv: "OPENAI_API_KEY", Models: []string{"gpt-4o", "gpt-4o-mini", "o3-mini"}},                                                          //nolint:gosec // env var name, not a credential
	{Name: "Ollama", Type: "ollama", BaseURL: config.KnownBaseURLs["ollama"], Supported: true, Models: []string{"llama3.3", "qwen3", "gemma3", "phi4", "deepseek-r1"}},
	{Name: "Mistral", Type: "mistral", APIKeyEnv: "MISTRAL_API_KEY", BaseURL: config.KnownBaseURLs["mistral"], Models: []string{"mistral-large-latest", "mistral-small-latest"}},   //nolint:gosec // env var name, not a credential
	{Name: "Regolo", Type: "regolo", APIKeyEnv: "REGOLO_API_KEY", BaseURL: config.KnownBaseURLs["regolo"], Models: []string{"deepseek-r1", "llama-4-maverick"}},                   //nolint:gosec // env var name, not a credential
}

// Result holds the outcome of a setup run for display purposes.
type Result struct {
	ProviderName string
	ProviderType string
	BaseURL      string
	Model        string
	ConfigPath   string
	EnvFilePath  string // path to .env file; empty if no secrets were stored
}

// Run executes the interactive first-run setup flow.
// It prompts the user for provider selection, provider-specific configuration,
// and model choice, then writes wildgecu.yaml and optionally .env to homeDir.
func Run(homeDir string, stdin io.Reader, stdout io.Writer, opts ...Option) (*Result, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	scanner := bufio.NewScanner(stdin)

	fmt.Fprint(stdout, "\nWelcome to wildgecu! Let's set up your LLM provider.\n\n")

	p, err := selectProvider(scanner, stdout)
	if err != nil {
		return nil, err
	}

	var apiKey string
	if p.APIKeyEnv != "" {
		apiKey, err = promptRequired(scanner, stdout, "API Key")
		if err != nil {
			return nil, err
		}
	}

	baseURL := p.BaseURL
	if p.BaseURL != "" {
		baseURL, err = promptDefault(scanner, stdout, "Base URL", p.BaseURL)
		if err != nil {
			return nil, err
		}
	}

	var googleSearch bool
	if p.Type == "gemini" {
		googleSearch, err = promptYesNo(scanner, stdout, "Enable Google Search", false)
		if err != nil {
			return nil, err
		}
	}

	// Validate credentials after collecting all provider config.
	if p.APIKeyEnv != "" && o.validate != nil {
		for {
			validateErr := o.validate(p.Type, apiKey, baseURL)
			if validateErr == nil {
				break
			}
			fmt.Fprintf(stdout, "\nValidation failed: %v\n", validateErr)
			fmt.Fprint(stdout, "Please try again or press Ctrl+C to abort.\n\n")
			apiKey, err = promptRequired(scanner, stdout, "API Key")
			if err != nil {
				return nil, err
			}
		}
	}

	model, err := selectModel(scanner, stdout, p)
	if err != nil {
		return nil, err
	}

	// Write .env first since config references it via env().
	var envFilePath string
	if p.APIKeyEnv != "" {
		envFilePath = filepath.Join(homeDir, ".env")
		if err := writeEnvFile(envFilePath, p.APIKeyEnv, apiKey); err != nil {
			return nil, err
		}
	}

	if err := writeConfig(homeDir, p.Type, baseURL, model, p.APIKeyEnv, googleSearch); err != nil {
		return nil, err
	}

	return &Result{
		ProviderName: p.Name,
		ProviderType: p.Type,
		BaseURL:      baseURL,
		Model:        model,
		ConfigPath:   filepath.Join(homeDir, "wildgecu.yaml"),
		EnvFilePath:  envFilePath,
	}, nil
}

func selectProvider(scanner *bufio.Scanner, stdout io.Writer) (*provider, error) {
	for {
		fmt.Fprintln(stdout, "Choose a provider:")
		for i, p := range providers {
			fmt.Fprintf(stdout, "  %d) %s\n", i+1, p.Name)
		}
		fmt.Fprintf(stdout, "\nProvider [1-%d]: ", len(providers))

		if !scanner.Scan() {
			return nil, fmt.Errorf("setup cancelled")
		}

		input := strings.TrimSpace(scanner.Text())
		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > len(providers) {
			fmt.Fprintf(stdout, "\nInvalid choice %q. Please enter a number between 1 and %d.\n\n", input, len(providers))
			continue
		}

		p := &providers[n-1]
		if !p.Supported {
			fmt.Fprintf(stdout, "\n%s is not yet supported in the setup wizard. Please choose another provider or configure manually.\n\n", p.Name)
			continue
		}

		return p, nil
	}
}

func promptDefault(scanner *bufio.Scanner, stdout io.Writer, label, defaultVal string) (string, error) {
	fmt.Fprintf(stdout, "%s [%s]: ", label, defaultVal)
	if !scanner.Scan() {
		return "", fmt.Errorf("setup cancelled")
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultVal, nil
	}
	return input, nil
}

func promptRequired(scanner *bufio.Scanner, stdout io.Writer, label string) (string, error) {
	for {
		fmt.Fprintf(stdout, "%s: ", label)
		if !scanner.Scan() {
			return "", fmt.Errorf("setup cancelled")
		}
		input := strings.TrimSpace(scanner.Text())
		if input != "" {
			return input, nil
		}
		fmt.Fprintf(stdout, "%s is required.\n", label)
	}
}

func promptYesNo(scanner *bufio.Scanner, stdout io.Writer, label string, defaultVal bool) (bool, error) {
	hint := "y/N"
	if defaultVal {
		hint = "Y/n"
	}
	fmt.Fprintf(stdout, "%s [%s]: ", label, hint)
	if !scanner.Scan() {
		return false, fmt.Errorf("setup cancelled")
	}
	input := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if input == "" {
		return defaultVal, nil
	}
	return input == "y" || input == "yes", nil
}

func selectModel(scanner *bufio.Scanner, stdout io.Writer, p *provider) (string, error) {
	fmt.Fprintln(stdout, "\nChoose a model:")
	for i, m := range p.Models {
		fmt.Fprintf(stdout, "  %d) %s\n", i+1, m)
	}
	fmt.Fprintln(stdout, "\nEnter a number to select, or type a custom model name.")
	fmt.Fprint(stdout, "Model [1]: ")

	if !scanner.Scan() {
		return "", fmt.Errorf("setup cancelled")
	}

	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return p.Models[0], nil
	}

	n, err := strconv.Atoi(input)
	if err == nil && n >= 1 && n <= len(p.Models) {
		return p.Models[n-1], nil
	}

	return input, nil
}

// yamlProviderConfig is used only for YAML serialization during setup.
type yamlProviderConfig struct {
	Type         string `yaml:"type"`
	BaseURL      string `yaml:"base_url,omitempty"`
	APIKey       string `yaml:"api_key,omitempty"`
	GoogleSearch bool   `yaml:"google_search,omitempty"`
}

// yamlConfig is used only for YAML serialization during setup.
type yamlConfig struct {
	Providers    map[string]yamlProviderConfig `yaml:"providers"`
	Models       map[string]string             `yaml:"models"`
	DefaultModel string                        `yaml:"default_model"`
}

func writeConfig(homeDir, providerType, baseURL, model, apiKeyEnv string, googleSearch bool) error {
	pc := yamlProviderConfig{
		Type:         providerType,
		BaseURL:      baseURL,
		GoogleSearch: googleSearch,
	}
	if apiKeyEnv != "" {
		pc.APIKey = "env(" + apiKeyEnv + ")"
	}

	cfg := yamlConfig{
		Providers: map[string]yamlProviderConfig{
			providerType: pc,
		},
		Models: map[string]string{
			"base": providerType + "/" + model,
		},
		DefaultModel: "base",
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := filepath.Join(homeDir, "wildgecu.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func writeEnvFile(path, envVarName, value string) error {
	envMap, err := godotenv.Read(path)
	if errors.Is(err, os.ErrNotExist) {
		envMap = map[string]string{}
	} else if err != nil {
		return fmt.Errorf("read existing .env: %w", err)
	}

	envMap[envVarName] = value

	if err := godotenv.Write(envMap, path); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}

	return nil
}

// FormatSummary returns a human-readable summary of the setup result.
func FormatSummary(r *Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\nConfiguration saved to %s\n\n", r.ConfigPath)
	fmt.Fprintf(&b, "  Provider: %s\n", r.ProviderName)
	if r.BaseURL != "" {
		fmt.Fprintf(&b, "  Base URL: %s\n", r.BaseURL)
	}
	fmt.Fprintf(&b, "  Model:    %s\n", r.Model)
	fmt.Fprintf(&b, "  Alias:    base -> %s/%s\n", r.ProviderType, r.Model)
	if r.EnvFilePath != "" {
		fmt.Fprintf(&b, "  Secrets:  %s\n", r.EnvFilePath)
	}
	return b.String()
}
