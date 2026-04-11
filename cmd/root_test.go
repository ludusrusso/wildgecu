package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"wildgecu/x/config"
)

func TestInitConfig(t *testing.T) {
	t.Run("LoadsFromGlobalHome", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		// Reset global state.
		config.SetGlobalHome("")
		homeFlag = ""
		appConfig = nil

		globalDir := filepath.Join(tmpHome, ".wildgecu")
		if err := os.MkdirAll(globalDir, 0o755); err != nil {
			t.Fatal(err)
		}

		cfgContent := []byte(`providers:
  gemini:
    type: gemini
    api_key: from-global-home
default_model: gemini/global-model
`)
		if err := os.WriteFile(filepath.Join(globalDir, "wildgecu.yaml"), cfgContent, 0o644); err != nil {
			t.Fatal(err)
		}

		initConfig()

		if appConfig == nil {
			t.Fatal("appConfig is nil after initConfig")
		}

		if appConfig.DefaultModel != "gemini/global-model" {
			t.Errorf("DefaultModel = %q, want %q", appConfig.DefaultModel, "gemini/global-model")
		}

		g, ok := appConfig.Providers["gemini"]
		if !ok {
			t.Fatal("gemini provider not found in config")
		}
		if g.APIKey != "from-global-home" {
			t.Errorf("gemini.APIKey = %q, want %q", g.APIKey, "from-global-home")
		}
	})

	t.Run("ModelFlagOverridesDefaultModel", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		config.SetGlobalHome("")
		homeFlag = ""
		modelFlag = ""
		appConfig = nil

		globalDir := filepath.Join(tmpHome, ".wildgecu")
		if err := os.MkdirAll(globalDir, 0o755); err != nil {
			t.Fatal(err)
		}

		cfgContent := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
  openai:
    type: openai
    api_key: key
models:
  fast: "gemini/flash"
  smart: "openai/gpt-4o"
default_model: fast
`)
		if err := os.WriteFile(filepath.Join(globalDir, "wildgecu.yaml"), cfgContent, 0o644); err != nil {
			t.Fatal(err)
		}

		modelFlag = "smart"
		initConfig()

		if appConfig.DefaultModel != "smart" {
			t.Errorf("DefaultModel = %q, want %q", appConfig.DefaultModel, "smart")
		}
	})

	t.Run("ModelFlagAcceptsDirectProviderModel", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		config.SetGlobalHome("")
		homeFlag = ""
		modelFlag = ""
		appConfig = nil

		globalDir := filepath.Join(tmpHome, ".wildgecu")
		if err := os.MkdirAll(globalDir, 0o755); err != nil {
			t.Fatal(err)
		}

		cfgContent := []byte(`providers:
  gemini:
    type: gemini
    api_key: key
default_model: gemini/flash
`)
		if err := os.WriteFile(filepath.Join(globalDir, "wildgecu.yaml"), cfgContent, 0o644); err != nil {
			t.Fatal(err)
		}

		modelFlag = "gemini/pro"
		initConfig()

		if appConfig.DefaultModel != "gemini/pro" {
			t.Errorf("DefaultModel = %q, want %q", appConfig.DefaultModel, "gemini/pro")
		}
	})

	t.Run("HomeOverrideLoadsDifferentConfig", func(t *testing.T) {
		tmpHome := t.TempDir()
		customHome := t.TempDir()

		t.Setenv("HOME", tmpHome)
		config.SetGlobalHome("")
		homeFlag = customHome
		modelFlag = ""
		appConfig = nil

		cfgContent := []byte(`providers:
  openai:
    type: openai
    api_key: custom-key
default_model: openai/gpt-4o
`)
		if err := os.WriteFile(filepath.Join(customHome, "wildgecu.yaml"), cfgContent, 0o644); err != nil {
			t.Fatal(err)
		}

		initConfig()

		if appConfig == nil {
			t.Fatal("appConfig is nil after initConfig")
		}

		if appConfig.DefaultModel != "openai/gpt-4o" {
			t.Errorf("DefaultModel = %q, want %q", appConfig.DefaultModel, "openai/gpt-4o")
		}
	})
}
