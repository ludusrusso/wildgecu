package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestEnsureConfigFile(t *testing.T) {
	t.Run("CreatesFileWhenMissing", func(t *testing.T) {
		dir := t.TempDir()
		SetGlobalHome(dir)
		t.Cleanup(func() { SetGlobalHome("") })

		path, created, err := EnsureConfigFile()
		if err != nil {
			t.Fatalf("EnsureConfigFile() error = %v", err)
		}
		if !created {
			t.Error("expected created=true for new file")
		}
		if filepath.Dir(path) != dir {
			t.Errorf("path dir = %q, want %q", filepath.Dir(path), dir)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if len(data) == 0 {
			t.Fatal("config file is empty")
		}
	})

	t.Run("SkipsExistingFile", func(t *testing.T) {
		dir := t.TempDir()
		SetGlobalHome(dir)
		t.Cleanup(func() { SetGlobalHome("") })

		existing := filepath.Join(dir, "wildgecu.yaml")
		if err := os.WriteFile(existing, []byte("custom"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, created, err := EnsureConfigFile()
		if err != nil {
			t.Fatalf("EnsureConfigFile() error = %v", err)
		}
		if created {
			t.Error("expected created=false for existing file")
		}

		data, err := os.ReadFile(existing)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "custom" {
			t.Error("existing file should not be overwritten")
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("IsValidYAML", func(t *testing.T) {
		var cfg Config
		if err := yaml.Unmarshal([]byte(defaultConfig), &cfg); err != nil {
			t.Fatalf("default config is not valid YAML: %v", err)
		}
	})

	t.Run("HasProviders", func(t *testing.T) {
		var cfg Config
		if err := yaml.Unmarshal([]byte(defaultConfig), &cfg); err != nil {
			t.Fatal(err)
		}
		if len(cfg.Providers) == 0 {
			t.Error("default config should have at least one provider")
		}
	})

	t.Run("HasDefaultModel", func(t *testing.T) {
		var cfg Config
		if err := yaml.Unmarshal([]byte(defaultConfig), &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.DefaultModel == "" {
			t.Error("default config should set default_model")
		}
	})

	t.Run("DefaultModelReferencesKnownProvider", func(t *testing.T) {
		var cfg Config
		if err := yaml.Unmarshal([]byte(defaultConfig), &cfg); err != nil {
			t.Fatal(err)
		}
		parts := strings.SplitN(cfg.DefaultModel, "/", 2)
		if len(parts) != 2 {
			t.Fatalf("default_model %q should be in provider/model format", cfg.DefaultModel)
		}
		if _, ok := cfg.Providers[parts[0]]; !ok {
			t.Errorf("default_model references provider %q which is not in providers", parts[0])
		}
	})

	t.Run("ShowsEnvSyntax", func(t *testing.T) {
		if !strings.Contains(defaultConfig, "env(") {
			t.Error("default config should demonstrate env() syntax for secrets")
		}
	})

	t.Run("ShowsModelsSection", func(t *testing.T) {
		if !strings.Contains(defaultConfig, "models:") {
			t.Error("default config should include a models section (commented or not)")
		}
	})
}
