package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	t.Run("LoadsVariablesFromFile", func(t *testing.T) {
		dir := t.TempDir()
		envFile := filepath.Join(dir, ".env")
		if err := os.WriteFile(envFile, []byte("DOTENV_TEST_VAR=hello\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := LoadDotEnv(dir); err != nil {
			t.Fatalf("LoadDotEnv() error = %v", err)
		}

		got := os.Getenv("DOTENV_TEST_VAR")
		if got != "hello" {
			t.Errorf("DOTENV_TEST_VAR = %q, want %q", got, "hello")
		}

		// Clean up so we don't leak into other tests.
		os.Unsetenv("DOTENV_TEST_VAR")
	})

	t.Run("SkipsMissingFile", func(t *testing.T) {
		dir := t.TempDir()

		if err := LoadDotEnv(dir); err != nil {
			t.Fatalf("LoadDotEnv() error = %v, want nil for missing .env", err)
		}
	})

	t.Run("RealEnvVarTakesPrecedence", func(t *testing.T) {
		dir := t.TempDir()
		envFile := filepath.Join(dir, ".env")
		if err := os.WriteFile(envFile, []byte("DOTENV_PREC_TEST=from-dotenv\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		t.Setenv("DOTENV_PREC_TEST", "from-real-env")

		if err := LoadDotEnv(dir); err != nil {
			t.Fatalf("LoadDotEnv() error = %v", err)
		}

		got := os.Getenv("DOTENV_PREC_TEST")
		if got != "from-real-env" {
			t.Errorf("DOTENV_PREC_TEST = %q, want %q (real env should win)", got, "from-real-env")
		}
	})
}
