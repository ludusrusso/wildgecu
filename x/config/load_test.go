package config

import (
	"os"
	"path/filepath"
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
}
