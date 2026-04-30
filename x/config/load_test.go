package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("ParsesValidConfig", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: test-gemini-key
    google_search: true
  openai:
    type: openai
    api_key: test-openai-key
  ollama:
    type: openai
    base_url: http://localhost:11434/v1
default_model: gemini/gemini-3-flash-preview
telegram_token: tg-token-123
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.DefaultModel != "gemini/gemini-3-flash-preview" {
			t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "gemini/gemini-3-flash-preview")
		}
		if cfg.TelegramToken != "tg-token-123" {
			t.Errorf("TelegramToken = %q, want %q", cfg.TelegramToken, "tg-token-123")
		}
		if len(cfg.Providers) != 3 {
			t.Fatalf("len(Providers) = %d, want 3", len(cfg.Providers))
		}

		g := cfg.Providers["gemini"]
		if g.Type != "gemini" {
			t.Errorf("gemini.Type = %q, want %q", g.Type, "gemini")
		}
		if g.APIKey != "test-gemini-key" {
			t.Errorf("gemini.APIKey = %q, want %q", g.APIKey, "test-gemini-key")
		}
		if !g.GoogleSearch {
			t.Error("gemini.GoogleSearch = false, want true")
		}

		o := cfg.Providers["openai"]
		if o.Type != "openai" {
			t.Errorf("openai.Type = %q, want %q", o.Type, "openai")
		}
		if o.APIKey != "test-openai-key" {
			t.Errorf("openai.APIKey = %q, want %q", o.APIKey, "test-openai-key")
		}

		ol := cfg.Providers["ollama"]
		if ol.BaseURL != "http://localhost:11434/v1" {
			t.Errorf("ollama.BaseURL = %q, want %q", ol.BaseURL, "http://localhost:11434/v1")
		}
	})

	t.Run("ErrorOnMissingFile", func(t *testing.T) {
		_, err := Load("/nonexistent/path/wildgecu.yaml")
		if err == nil {
			t.Error("Load() expected error for missing file, got nil")
		}
	})

	t.Run("ErrorOnMissingDefaultModel", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(cfgPath)
		if err == nil {
			t.Error("Load() expected error for missing default_model, got nil")
		}
	})

	t.Run("ErrorOnEmptyProviders", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers: {}
default_model: gemini/gemini-3-flash-preview
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(cfgPath)
		if err == nil {
			t.Error("Load() expected error for empty providers, got nil")
		}
	})

	t.Run("ErrorOnInvalidDefaultModelFormat", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
default_model: just-a-model-name
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(cfgPath)
		if err == nil {
			t.Error("Load() expected error for invalid default_model format (no slash), got nil")
		}
	})

	t.Run("ErrorOnDefaultModelReferencingUnknownProvider", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
default_model: unknown/some-model
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(cfgPath)
		if err == nil {
			t.Error("Load() expected error for default_model referencing unknown provider, got nil")
		}
	})

	t.Run("ErrorOnProviderMissingType", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  mygemini:
    api_key: key
default_model: mygemini/some-model
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(cfgPath)
		if err == nil {
			t.Error("Load() expected error for provider missing type, got nil")
		}
	})

	t.Run("EnvResolvesAPIKey", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: env(GEMINI_API_KEY)
default_model: gemini/gemini-3-flash-preview
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		t.Setenv("GEMINI_API_KEY", "secret-from-env")

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Providers["gemini"].APIKey != "secret-from-env" {
			t.Errorf("gemini.APIKey = %q, want %q", cfg.Providers["gemini"].APIKey, "secret-from-env")
		}
	})

	t.Run("ErrorOnMissingEnvVar", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  google:
    type: gemini
    api_key: env(MISSING_SECRET_VAR)
default_model: google/gemini-3-flash-preview
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(cfgPath)
		if err == nil {
			t.Fatal("Load() expected error for missing env var, got nil")
		}

		errMsg := err.Error()
		if !strings.Contains(errMsg, "google") {
			t.Errorf("error should name provider %q, got: %s", "google", errMsg)
		}
		if !strings.Contains(errMsg, "MISSING_SECRET_VAR") {
			t.Errorf("error should name env var %q, got: %s", "MISSING_SECRET_VAR", errMsg)
		}
	})

	t.Run("EnvResolvesMultipleFields", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		// Uses both quoted and unquoted env() syntax
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: env(TEST_GEMINI_KEY)
  ollama:
    type: openai
    base_url: "env(TEST_OLLAMA_URL)"
default_model: gemini/gemini-3-flash-preview
telegram_token: env(TEST_TG_TOKEN)
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		t.Setenv("TEST_GEMINI_KEY", "gemini-secret")
		t.Setenv("TEST_OLLAMA_URL", "http://my-ollama:11434/v1")
		t.Setenv("TEST_TG_TOKEN", "tg-secret")

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Providers["gemini"].APIKey != "gemini-secret" {
			t.Errorf("gemini.APIKey = %q, want %q", cfg.Providers["gemini"].APIKey, "gemini-secret")
		}
		if cfg.Providers["ollama"].BaseURL != "http://my-ollama:11434/v1" {
			t.Errorf("ollama.BaseURL = %q, want %q", cfg.Providers["ollama"].BaseURL, "http://my-ollama:11434/v1")
		}
		if cfg.TelegramToken != "tg-secret" {
			t.Errorf("TelegramToken = %q, want %q", cfg.TelegramToken, "tg-secret")
		}
	})

	t.Run("LiteralStringsUnchanged", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: plain-api-key
  ollama:
    type: openai
    base_url: http://localhost:11434/v1
default_model: gemini/gemini-3-flash-preview
telegram_token: my-plain-token
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Providers["gemini"].APIKey != "plain-api-key" {
			t.Errorf("gemini.APIKey = %q, want %q", cfg.Providers["gemini"].APIKey, "plain-api-key")
		}
		if cfg.Providers["ollama"].BaseURL != "http://localhost:11434/v1" {
			t.Errorf("ollama.BaseURL = %q, want %q", cfg.Providers["ollama"].BaseURL, "http://localhost:11434/v1")
		}
		if cfg.TelegramToken != "my-plain-token" {
			t.Errorf("TelegramToken = %q, want %q", cfg.TelegramToken, "my-plain-token")
		}
	})

	t.Run("SugarDefaultsApplied", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  mistral:
    type: mistral
    api_key: mk
  regolo:
    type: regolo
    api_key: rk
  ollama:
    type: ollama
default_model: mistral/mistral-large
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		tests := []struct {
			name    string
			wantURL string
		}{
			{"mistral", "https://api.mistral.ai/v1"},
			{"regolo", "https://api.regolo.ai/v1"},
			{"ollama", "http://localhost:11434/v1"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p := cfg.Providers[tt.name]
				if p.BaseURL != tt.wantURL {
					t.Errorf("%s.BaseURL = %q, want %q", tt.name, p.BaseURL, tt.wantURL)
				}
			})
		}
	})

	t.Run("ExplicitBaseURLOverridesSugar", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  custom-regolo:
    type: regolo
    api_key: rk
    base_url: https://custom.regolo.endpoint/v1
default_model: custom-regolo/some-model
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		p := cfg.Providers["custom-regolo"]
		if p.BaseURL != "https://custom.regolo.endpoint/v1" {
			t.Errorf("BaseURL = %q, want %q", p.BaseURL, "https://custom.regolo.endpoint/v1")
		}
	})

	t.Run("SugarDoesNotAffectOpenAIOrGemini", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  openai:
    type: openai
    api_key: ok
  gemini:
    type: gemini
    api_key: gk
default_model: openai/gpt-4
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Providers["openai"].BaseURL != "" {
			t.Errorf("openai.BaseURL = %q, want empty", cfg.Providers["openai"].BaseURL)
		}
		if cfg.Providers["gemini"].BaseURL != "" {
			t.Errorf("gemini.BaseURL = %q, want empty", cfg.Providers["gemini"].BaseURL)
		}
	})

	t.Run("ParsesModelsMap", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  local:
    type: ollama
  openai:
    type: openai
    api_key: key
models:
  fast: "local/llama3"
  smart: "openai/gpt-4o"
default_model: smart
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(cfg.Models) != 2 {
			t.Fatalf("len(Models) = %d, want 2", len(cfg.Models))
		}
		if cfg.Models["fast"] != "local/llama3" {
			t.Errorf("Models[fast] = %q, want %q", cfg.Models["fast"], "local/llama3")
		}
		if cfg.Models["smart"] != "openai/gpt-4o" {
			t.Errorf("Models[smart] = %q, want %q", cfg.Models["smart"], "openai/gpt-4o")
		}
		if cfg.DefaultModel != "smart" {
			t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "smart")
		}
	})

	t.Run("ErrorOnAliasNameContainingSlash", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
models:
  "bad/alias": "gemini/flash"
default_model: gemini/flash
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(cfgPath)
		if err == nil {
			t.Error("Load() expected error for alias containing slash, got nil")
		}
		if !strings.Contains(err.Error(), "bad/alias") {
			t.Errorf("error should mention alias name, got: %s", err.Error())
		}
	})

	t.Run("ErrorOnAliasValueInvalidFormat", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
models:
  broken: "no-slash"
default_model: gemini/flash
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(cfgPath)
		if err == nil {
			t.Error("Load() expected error for alias value without provider/model format, got nil")
		}
	})

	t.Run("ErrorOnAliasReferencingUnknownProvider", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
models:
  myalias: "unknown/model"
default_model: gemini/flash
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(cfgPath)
		if err == nil {
			t.Error("Load() expected error for alias referencing unknown provider, got nil")
		}
	})

	t.Run("DefaultModelAcceptsAlias", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
models:
  fast: "gemini/flash"
default_model: fast
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.DefaultModel != "fast" {
			t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "fast")
		}
	})

	t.Run("DefaultModelAcceptsDirectProviderModel", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
default_model: gemini/flash
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.DefaultModel != "gemini/flash" {
			t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "gemini/flash")
		}
	})

	t.Run("ErrorOnDefaultModelUnknownAlias", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
models:
  fast: "gemini/flash"
default_model: nonexistent
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(cfgPath)
		if err == nil {
			t.Error("Load() expected error for default_model referencing unknown alias, got nil")
		}
	})

	t.Run("EmptyModelsMapIsValid", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
default_model: gemini/flash
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(cfg.Models) != 0 {
			t.Errorf("Models should be nil or empty, got %v", cfg.Models)
		}
	})

	t.Run("ValidateModelRefAcceptsDirectProviderModel", func(t *testing.T) {
		cfg := &Config{
			Providers: map[string]ProviderConfig{
				"gemini": {Type: "gemini", APIKey: "key"},
			},
			Models: map[string]string{},
		}
		if err := cfg.ValidateModelRef("gemini/flash"); err != nil {
			t.Errorf("ValidateModelRef() unexpected error = %v", err)
		}
	})

	t.Run("ValidateModelRefAcceptsKnownAlias", func(t *testing.T) {
		cfg := &Config{
			Providers: map[string]ProviderConfig{
				"gemini": {Type: "gemini", APIKey: "key"},
			},
			Models: map[string]string{
				"fast": "gemini/flash",
			},
		}
		if err := cfg.ValidateModelRef("fast"); err != nil {
			t.Errorf("ValidateModelRef() unexpected error = %v", err)
		}
	})

	t.Run("ValidateModelRefRejectsUnknownAlias", func(t *testing.T) {
		cfg := &Config{
			Providers: map[string]ProviderConfig{
				"gemini": {Type: "gemini", APIKey: "key"},
			},
			Models: map[string]string{
				"fast": "gemini/flash",
			},
		}
		if err := cfg.ValidateModelRef("nonexistent"); err == nil {
			t.Error("ValidateModelRef() expected error for unknown alias, got nil")
		}
	})

	t.Run("ValidateModelRefRejectsUnknownProvider", func(t *testing.T) {
		cfg := &Config{
			Providers: map[string]ProviderConfig{
				"gemini": {Type: "gemini", APIKey: "key"},
			},
			Models: map[string]string{},
		}
		if err := cfg.ValidateModelRef("openai/gpt-4o"); err == nil {
			t.Error("ValidateModelRef() expected error for unknown provider, got nil")
		}
	})

	t.Run("UnknownFieldsIgnored", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
    some_random_field: whatever
    another_thing: 42
default_model: gemini/gemini-3-flash-preview
extra_top_level: ignored
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v; unknown fields should be silently ignored", err)
		}
		if cfg.DefaultModel != "gemini/gemini-3-flash-preview" {
			t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "gemini/gemini-3-flash-preview")
		}
	})

	t.Run("ParsesToolsBlock", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
default_model: gemini/gemini-3-flash-preview
tools:
  grep:
    max_results: 50
    max_file_size_bytes: 524288
  bash:
    max_timeout_seconds: 300
    head_bytes: 16384
    tail_bytes: 4096
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Tools.Grep.MaxResults != 50 {
			t.Errorf("Tools.Grep.MaxResults = %d, want 50", cfg.Tools.Grep.MaxResults)
		}
		if cfg.Tools.Grep.MaxFileSizeBytes != 524288 {
			t.Errorf("Tools.Grep.MaxFileSizeBytes = %d, want 524288", cfg.Tools.Grep.MaxFileSizeBytes)
		}
		if cfg.Tools.Bash.MaxTimeoutSeconds != 300 {
			t.Errorf("Tools.Bash.MaxTimeoutSeconds = %d, want 300", cfg.Tools.Bash.MaxTimeoutSeconds)
		}
		if cfg.Tools.Bash.HeadBytes != 16384 {
			t.Errorf("Tools.Bash.HeadBytes = %d, want 16384", cfg.Tools.Bash.HeadBytes)
		}
		if cfg.Tools.Bash.TailBytes != 4096 {
			t.Errorf("Tools.Bash.TailBytes = %d, want 4096", cfg.Tools.Bash.TailBytes)
		}
	})

	t.Run("ToolsBlockOptional", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "wildgecu.yaml")
		data := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
default_model: gemini/gemini-3-flash-preview
`)
		if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		// Zero values are fine; defaults are applied at the tool layer.
		if cfg.Tools.Grep.MaxResults != 0 {
			t.Errorf("Tools.Grep.MaxResults default = %d, want 0", cfg.Tools.Grep.MaxResults)
		}
	})
}
