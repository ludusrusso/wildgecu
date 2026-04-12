package setup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ludusrusso/wildgecu/x/config"
	"go.yaml.in/yaml/v3"
)

// provider describes a configurable LLM provider.
type provider struct {
	Name    string
	Type    string
	Models  []string
	BaseURL string // default base URL; empty if not applicable
}

var providers = []provider{
	{Name: "Gemini", Type: "gemini", Models: []string{"gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.0-flash"}},
	{Name: "OpenAI", Type: "openai", Models: []string{"gpt-4o", "gpt-4o-mini", "o3-mini"}},
	{Name: "Ollama", Type: "ollama", BaseURL: config.KnownBaseURLs["ollama"], Models: []string{"llama3.3", "qwen3", "gemma3", "phi4", "deepseek-r1"}},
	{Name: "Mistral", Type: "mistral", BaseURL: config.KnownBaseURLs["mistral"], Models: []string{"mistral-large-latest", "mistral-small-latest"}},
	{Name: "Regolo", Type: "regolo", BaseURL: config.KnownBaseURLs["regolo"], Models: []string{"deepseek-r1", "llama-4-maverick"}},
}

// supportedTypes lists provider types fully implemented in the setup wizard.
var supportedTypes = map[string]bool{
	"ollama": true,
}

// Result holds the outcome of a setup run for display purposes.
type Result struct {
	ProviderName string
	ProviderType string
	BaseURL      string
	Model        string
	ConfigPath   string
}

// Run executes the interactive first-run setup flow.
// It prompts the user for provider selection, provider-specific configuration,
// and model choice, then writes wildgecu.yaml to homeDir.
func Run(homeDir string, stdin io.Reader, stdout io.Writer) (*Result, error) {
	scanner := bufio.NewScanner(stdin)

	fmt.Fprint(stdout, "\nWelcome to wildgecu! Let's set up your LLM provider.\n\n")

	p, err := selectProvider(scanner, stdout)
	if err != nil {
		return nil, err
	}

	baseURL := p.BaseURL
	if p.BaseURL != "" {
		baseURL, err = promptDefault(scanner, stdout, "Base URL", p.BaseURL)
		if err != nil {
			return nil, err
		}
	}

	model, err := selectModel(scanner, stdout, p)
	if err != nil {
		return nil, err
	}

	if err := writeConfig(homeDir, p.Type, baseURL, model); err != nil {
		return nil, err
	}

	return &Result{
		ProviderName: p.Name,
		ProviderType: p.Type,
		BaseURL:      baseURL,
		Model:        model,
		ConfigPath:   filepath.Join(homeDir, "wildgecu.yaml"),
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
		if !supportedTypes[p.Type] {
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
	Type    string `yaml:"type"`
	BaseURL string `yaml:"base_url,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"`
}

// yamlConfig is used only for YAML serialization during setup.
type yamlConfig struct {
	Providers    map[string]yamlProviderConfig `yaml:"providers"`
	Models       map[string]string             `yaml:"models"`
	DefaultModel string                        `yaml:"default_model"`
}

func writeConfig(homeDir, providerType, baseURL, model string) error {
	cfg := yamlConfig{
		Providers: map[string]yamlProviderConfig{
			providerType: {
				Type:    providerType,
				BaseURL: baseURL,
			},
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
	return b.String()
}
