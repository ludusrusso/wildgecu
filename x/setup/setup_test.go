package setup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/ludusrusso/wildgecu/x/config"
)

func TestRun(t *testing.T) {
	t.Run("OllamaWithDefaults", func(t *testing.T) {
		homeDir := t.TempDir()
		// Select Ollama (provider 3), accept default base URL (empty), accept default model (empty).
		stdin := strings.NewReader("3\n\n\n")
		var stdout bytes.Buffer

		result, err := Run(homeDir, stdin, &stdout)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if result.ProviderType != "ollama" {
			t.Errorf("ProviderType = %q, want %q", result.ProviderType, "ollama")
		}
		if result.BaseURL != "http://localhost:11434/v1" {
			t.Errorf("BaseURL = %q, want %q", result.BaseURL, "http://localhost:11434/v1")
		}
		if result.Model != "llama3.3" {
			t.Errorf("Model = %q, want %q", result.Model, "llama3.3")
		}
		if result.EnvFilePath != "" {
			t.Errorf("EnvFilePath = %q, want empty (Ollama has no API key)", result.EnvFilePath)
		}
		// Verify the written config can be loaded.
		cfg := loadTestConfig(t, homeDir)

		if cfg.DefaultModel != "base" {
			t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "base")
		}
		if cfg.Models["base"] != "ollama/llama3.3" {
			t.Errorf("Models[base] = %q, want %q", cfg.Models["base"], "ollama/llama3.3")
		}

		p, ok := cfg.Providers["ollama"]
		if !ok {
			t.Fatal("provider 'ollama' not found in config")
		}
		if p.Type != "ollama" {
			t.Errorf("ollama.Type = %q, want %q", p.Type, "ollama")
		}
		if p.BaseURL != "http://localhost:11434/v1" {
			t.Errorf("ollama.BaseURL = %q, want %q", p.BaseURL, "http://localhost:11434/v1")
		}
	})

	t.Run("OllamaWithCustomBaseURL", func(t *testing.T) {
		homeDir := t.TempDir()
		// Select Ollama (3), custom base URL, accept default model.
		stdin := strings.NewReader("3\nhttp://myhost:11434/v1\n\n")
		var stdout bytes.Buffer

		result, err := Run(homeDir, stdin, &stdout)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if result.BaseURL != "http://myhost:11434/v1" {
			t.Errorf("BaseURL = %q, want %q", result.BaseURL, "http://myhost:11434/v1")
		}

		cfg := loadTestConfig(t, homeDir)
		if cfg.Providers["ollama"].BaseURL != "http://myhost:11434/v1" {
			t.Errorf("config BaseURL = %q, want %q", cfg.Providers["ollama"].BaseURL, "http://myhost:11434/v1")
		}
	})

	t.Run("OllamaWithModelByNumber", func(t *testing.T) {
		homeDir := t.TempDir()
		// Select Ollama (3), accept default base URL, pick model #3 (gemma3).
		stdin := strings.NewReader("3\n\n3\n")
		var stdout bytes.Buffer

		result, err := Run(homeDir, stdin, &stdout)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if result.Model != "gemma3" {
			t.Errorf("Model = %q, want %q", result.Model, "gemma3")
		}

		cfg := loadTestConfig(t, homeDir)
		if cfg.Models["base"] != "ollama/gemma3" {
			t.Errorf("Models[base] = %q, want %q", cfg.Models["base"], "ollama/gemma3")
		}
	})

	t.Run("OllamaWithCustomModelName", func(t *testing.T) {
		homeDir := t.TempDir()
		// Select Ollama (3), accept default base URL, type custom model name.
		stdin := strings.NewReader("3\n\nmy-custom-model\n")
		var stdout bytes.Buffer

		result, err := Run(homeDir, stdin, &stdout)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if result.Model != "my-custom-model" {
			t.Errorf("Model = %q, want %q", result.Model, "my-custom-model")
		}

		cfg := loadTestConfig(t, homeDir)
		if cfg.Models["base"] != "ollama/my-custom-model" {
			t.Errorf("Models[base] = %q, want %q", cfg.Models["base"], "ollama/my-custom-model")
		}
	})

	// Table-driven tests for API-key-based providers (OpenAI, Mistral, Regolo).
	apiKeyProviders := []struct {
		name         string
		choice       string // provider menu number
		providerType string
		apiKey       string
		envVar       string
		baseURL      string // expected default; empty means no base URL prompt
		model        string // expected default model
	}{
		{"OpenAI", "2", "openai", "oai-key-123", "OPENAI_API_KEY", "", "gpt-4o"},
		{"Mistral", "4", "mistral", "mistral-key-123", "MISTRAL_API_KEY", "https://api.mistral.ai/v1", "mistral-large-latest"},
		{"Regolo", "5", "regolo", "regolo-key-123", "REGOLO_API_KEY", "https://api.regolo.ai/v1", "deepseek-r1"},
	}

	for _, tc := range apiKeyProviders {
		t.Run(tc.name+"WithDefaults", func(t *testing.T) {
			homeDir := t.TempDir()
			// Build stdin: provider choice, API key, [accept default base URL], accept default model.
			input := tc.choice + "\n" + tc.apiKey + "\n"
			if tc.baseURL != "" {
				input += "\n" // accept default base URL
			}
			input += "\n" // accept default model
			var stdout bytes.Buffer

			nopValidator := func(_, _, _ string) error { return nil }
			result, err := Run(homeDir, strings.NewReader(input), &stdout, WithValidator(nopValidator))
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			if result.ProviderType != tc.providerType {
				t.Errorf("ProviderType = %q, want %q", result.ProviderType, tc.providerType)
			}
			if result.BaseURL != tc.baseURL {
				t.Errorf("BaseURL = %q, want %q", result.BaseURL, tc.baseURL)
			}
			if result.Model != tc.model {
				t.Errorf("Model = %q, want %q", result.Model, tc.model)
			}
			if result.EnvFilePath == "" {
				t.Error("EnvFilePath should not be empty")
			}

			// Verify .env file content.
			envMap, err := godotenv.Read(filepath.Join(homeDir, ".env"))
			if err != nil {
				t.Fatalf("read .env: %v", err)
			}
			if envMap[tc.envVar] != tc.apiKey {
				t.Errorf("%s = %q, want %q", tc.envVar, envMap[tc.envVar], tc.apiKey)
			}

			// Verify YAML contains env() reference, not raw key.
			data, err := os.ReadFile(filepath.Join(homeDir, "wildgecu.yaml"))
			if err != nil {
				t.Fatalf("read config: %v", err)
			}
			envRef := "env(" + tc.envVar + ")"
			if !strings.Contains(string(data), envRef) {
				t.Errorf("YAML should contain %s, got:\n%s", envRef, data)
			}
			if strings.Contains(string(data), tc.apiKey) {
				t.Error("YAML should not contain raw API key")
			}

			// Verify config loads with env var set.
			t.Setenv(tc.envVar, tc.apiKey)
			cfg := loadTestConfig(t, homeDir)
			wantModel := tc.providerType + "/" + tc.model
			if cfg.Models["base"] != wantModel {
				t.Errorf("Models[base] = %q, want %q", cfg.Models["base"], wantModel)
			}
			p, ok := cfg.Providers[tc.providerType]
			if !ok {
				t.Fatalf("provider %q not found in config", tc.providerType)
			}
			if p.Type != tc.providerType {
				t.Errorf("Type = %q, want %q", p.Type, tc.providerType)
			}
			if p.APIKey != tc.apiKey {
				t.Errorf("APIKey = %q, want %q (resolved)", p.APIKey, tc.apiKey)
			}
			if p.BaseURL != tc.baseURL {
				t.Errorf("BaseURL = %q, want %q", p.BaseURL, tc.baseURL)
			}
		})
	}

	// Table-driven tests for custom base URL override (Mistral, Regolo).
	baseURLProviders := []struct {
		name         string
		choice       string
		providerType string
		apiKey       string
		envVar       string
		customURL    string
	}{
		{"Mistral", "4", "mistral", "mistral-key", "MISTRAL_API_KEY", "https://custom.mistral.ai/v1"},
		{"Regolo", "5", "regolo", "regolo-key", "REGOLO_API_KEY", "https://custom.regolo.ai/v1"},
	}

	for _, tc := range baseURLProviders {
		t.Run(tc.name+"WithCustomBaseURL", func(t *testing.T) {
			homeDir := t.TempDir()
			input := tc.choice + "\n" + tc.apiKey + "\n" + tc.customURL + "\n\n"
			var stdout bytes.Buffer

			nopValidator := func(_, _, _ string) error { return nil }
			result, err := Run(homeDir, strings.NewReader(input), &stdout, WithValidator(nopValidator))
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			if result.BaseURL != tc.customURL {
				t.Errorf("BaseURL = %q, want %q", result.BaseURL, tc.customURL)
			}

			t.Setenv(tc.envVar, tc.apiKey)
			cfg := loadTestConfig(t, homeDir)
			if cfg.Providers[tc.providerType].BaseURL != tc.customURL {
				t.Errorf("config BaseURL = %q, want %q", cfg.Providers[tc.providerType].BaseURL, tc.customURL)
			}
		})
	}

	t.Run("InvalidProviderChoiceThenOllama", func(t *testing.T) {
		homeDir := t.TempDir()
		// Invalid input "abc", then Ollama (3), accept defaults.
		stdin := strings.NewReader("abc\n3\n\n\n")
		var stdout bytes.Buffer

		result, err := Run(homeDir, stdin, &stdout)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if result.ProviderType != "ollama" {
			t.Errorf("ProviderType = %q, want %q", result.ProviderType, "ollama")
		}

		output := stdout.String()
		if !strings.Contains(output, "Invalid choice") {
			t.Error("expected 'Invalid choice' message for bad input")
		}
	})

	t.Run("ConfigFileIsValidYAML", func(t *testing.T) {
		homeDir := t.TempDir()
		stdin := strings.NewReader("3\n\n\n")
		var stdout bytes.Buffer

		_, err := Run(homeDir, stdin, &stdout)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		cfgPath := filepath.Join(homeDir, "wildgecu.yaml")
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if len(data) == 0 {
			t.Fatal("config file is empty")
		}
	})

	t.Run("EOFDuringProviderSelection", func(t *testing.T) {
		homeDir := t.TempDir()
		stdin := strings.NewReader("") // immediate EOF
		var stdout bytes.Buffer

		_, err := Run(homeDir, stdin, &stdout)
		if err == nil {
			t.Fatal("Run() expected error on EOF, got nil")
		}
		if !strings.Contains(err.Error(), "cancelled") {
			t.Errorf("error = %q, want it to contain 'cancelled'", err.Error())
		}
	})

	t.Run("EOFDuringModelSelection", func(t *testing.T) {
		homeDir := t.TempDir()
		// Select Ollama, accept base URL, then EOF during model selection.
		stdin := strings.NewReader("3\n\n")
		var stdout bytes.Buffer

		_, err := Run(homeDir, stdin, &stdout)
		if err == nil {
			t.Fatal("Run() expected error on EOF, got nil")
		}
		if !strings.Contains(err.Error(), "cancelled") {
			t.Errorf("error = %q, want it to contain 'cancelled'", err.Error())
		}
	})

	t.Run("GeminiWithDefaults", func(t *testing.T) {
		homeDir := t.TempDir()
		// Select Gemini (1), API key, no google_search, default model.
		stdin := strings.NewReader("1\ntest-key-123\n\n\n")
		var stdout bytes.Buffer

		validator := func(_, _, _ string) error { return nil }

		result, err := Run(homeDir, stdin, &stdout, WithValidator(validator))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if result.ProviderType != "gemini" {
			t.Errorf("ProviderType = %q, want %q", result.ProviderType, "gemini")
		}
		if result.Model != "gemini-2.5-flash" {
			t.Errorf("Model = %q, want %q", result.Model, "gemini-2.5-flash")
		}
		if result.EnvFilePath == "" {
			t.Error("EnvFilePath should not be empty for Gemini")
		}

		// Verify .env file content.
		envMap, err := godotenv.Read(filepath.Join(homeDir, ".env"))
		if err != nil {
			t.Fatalf("read .env: %v", err)
		}
		if envMap["GEMINI_API_KEY"] != "test-key-123" {
			t.Errorf("GEMINI_API_KEY = %q, want %q", envMap["GEMINI_API_KEY"], "test-key-123")
		}

		// Verify YAML contains env() reference, not raw key.
		data, err := os.ReadFile(filepath.Join(homeDir, "wildgecu.yaml"))
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if !strings.Contains(string(data), "env(GEMINI_API_KEY)") {
			t.Errorf("YAML should contain env(GEMINI_API_KEY), got:\n%s", data)
		}
		if strings.Contains(string(data), "test-key-123") {
			t.Error("YAML should not contain raw API key")
		}

		// Verify config loads with env var set.
		t.Setenv("GEMINI_API_KEY", "test-key-123")
		cfg := loadTestConfig(t, homeDir)
		if cfg.DefaultModel != "base" {
			t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "base")
		}
		if cfg.Models["base"] != "gemini/gemini-2.5-flash" {
			t.Errorf("Models[base] = %q, want %q", cfg.Models["base"], "gemini/gemini-2.5-flash")
		}

		p, ok := cfg.Providers["gemini"]
		if !ok {
			t.Fatal("provider 'gemini' not found in config")
		}
		if p.Type != "gemini" {
			t.Errorf("gemini.Type = %q, want %q", p.Type, "gemini")
		}
		if p.APIKey != "test-key-123" {
			t.Errorf("gemini.APIKey = %q, want %q (resolved)", p.APIKey, "test-key-123")
		}
		if p.GoogleSearch {
			t.Error("gemini.GoogleSearch should be false")
		}
	})

	t.Run("GeminiWithGoogleSearch", func(t *testing.T) {
		homeDir := t.TempDir()
		// Gemini (1), API key, enable google_search (y), default model.
		stdin := strings.NewReader("1\ntest-key-456\ny\n\n")
		var stdout bytes.Buffer

		validator := func(_, _, _ string) error { return nil }

		result, err := Run(homeDir, stdin, &stdout, WithValidator(validator))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if result.ProviderType != "gemini" {
			t.Errorf("ProviderType = %q, want %q", result.ProviderType, "gemini")
		}

		// Verify YAML has google_search.
		data, err := os.ReadFile(filepath.Join(homeDir, "wildgecu.yaml"))
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if !strings.Contains(string(data), "google_search") {
			t.Errorf("YAML should contain google_search, got:\n%s", data)
		}

		t.Setenv("GEMINI_API_KEY", "test-key-456")
		cfg := loadTestConfig(t, homeDir)
		if !cfg.Providers["gemini"].GoogleSearch {
			t.Error("gemini.GoogleSearch should be true")
		}
	})

	t.Run("GeminiValidationFailureThenRetry", func(t *testing.T) {
		homeDir := t.TempDir()
		// Gemini (1), bad key, no google_search, (validation fails),
		// good key (validation succeeds), default model.
		stdin := strings.NewReader("1\nbad-key\n\ngood-key\n\n")
		var stdout bytes.Buffer

		calls := 0
		validator := func(_, apiKey, _ string) error {
			calls++
			if apiKey == "bad-key" {
				return fmt.Errorf("invalid API key")
			}
			return nil
		}

		result, err := Run(homeDir, stdin, &stdout, WithValidator(validator))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if calls != 2 {
			t.Errorf("validator called %d times, want 2", calls)
		}

		output := stdout.String()
		if !strings.Contains(output, "Validation failed") {
			t.Error("output should contain 'Validation failed'")
		}
		if !strings.Contains(output, "invalid API key") {
			t.Error("output should contain the validation error message")
		}

		// Verify the good key was stored.
		envMap, err := godotenv.Read(filepath.Join(homeDir, ".env"))
		if err != nil {
			t.Fatalf("read .env: %v", err)
		}
		if envMap["GEMINI_API_KEY"] != "good-key" {
			t.Errorf("GEMINI_API_KEY = %q, want %q", envMap["GEMINI_API_KEY"], "good-key")
		}

		if result.Model != "gemini-2.5-flash" {
			t.Errorf("Model = %q, want %q", result.Model, "gemini-2.5-flash")
		}
	})

	t.Run("GeminiEOFDuringAPIKeyPrompt", func(t *testing.T) {
		homeDir := t.TempDir()
		// Select Gemini (1), then EOF during API key prompt.
		stdin := strings.NewReader("1\n")
		var stdout bytes.Buffer

		_, err := Run(homeDir, stdin, &stdout)
		if err == nil {
			t.Fatal("Run() expected error on EOF, got nil")
		}
		if !strings.Contains(err.Error(), "cancelled") {
			t.Errorf("error = %q, want it to contain 'cancelled'", err.Error())
		}
	})

	t.Run("GeminiNoValidatorSkipsValidation", func(t *testing.T) {
		homeDir := t.TempDir()
		// Gemini (1), API key, no google_search, default model. No validator passed.
		stdin := strings.NewReader("1\nany-key\n\n\n")
		var stdout bytes.Buffer

		result, err := Run(homeDir, stdin, &stdout)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if result.ProviderType != "gemini" {
			t.Errorf("ProviderType = %q, want %q", result.ProviderType, "gemini")
		}

		// Verify .env was still written.
		envMap, err := godotenv.Read(filepath.Join(homeDir, ".env"))
		if err != nil {
			t.Fatalf("read .env: %v", err)
		}
		if envMap["GEMINI_API_KEY"] != "any-key" {
			t.Errorf("GEMINI_API_KEY = %q, want %q", envMap["GEMINI_API_KEY"], "any-key")
		}
	})
}

func TestFormatSummary(t *testing.T) {
	t.Run("IncludesAllFields", func(t *testing.T) {
		r := &Result{
			ProviderName: "Ollama",
			ProviderType: "ollama",
			BaseURL:      "http://localhost:11434/v1",
			Model:        "llama3.3",
			ConfigPath:   "/home/user/.wildgecu/wildgecu.yaml",
		}

		summary := FormatSummary(r)

		for _, want := range []string{"Ollama", "http://localhost:11434/v1", "llama3.3", "ollama/llama3.3", "/home/user/.wildgecu/wildgecu.yaml"} {
			if !strings.Contains(summary, want) {
				t.Errorf("summary missing %q:\n%s", want, summary)
			}
		}
	})

	t.Run("OmitsBaseURLWhenEmpty", func(t *testing.T) {
		r := &Result{
			ProviderName: "Gemini",
			ProviderType: "gemini",
			Model:        "gemini-2.5-flash",
			ConfigPath:   "/tmp/wildgecu.yaml",
		}

		summary := FormatSummary(r)
		if strings.Contains(summary, "Base URL") {
			t.Error("summary should not include Base URL when empty")
		}
	})

	t.Run("IncludesSecretsPath", func(t *testing.T) {
		r := &Result{
			ProviderName: "Gemini",
			ProviderType: "gemini",
			Model:        "gemini-2.5-flash",
			ConfigPath:   "/home/user/.wildgecu/wildgecu.yaml",
			EnvFilePath:  "/home/user/.wildgecu/.env",
		}

		summary := FormatSummary(r)
		if !strings.Contains(summary, "/home/user/.wildgecu/.env") {
			t.Errorf("summary missing env file path:\n%s", summary)
		}
		if !strings.Contains(summary, "Secrets") {
			t.Errorf("summary missing 'Secrets' label:\n%s", summary)
		}
	})
}

// loadTestConfig loads and validates the written config from homeDir.
func loadTestConfig(t *testing.T, homeDir string) *config.Config {
	t.Helper()
	cfgPath := filepath.Join(homeDir, "wildgecu.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return cfg
}
