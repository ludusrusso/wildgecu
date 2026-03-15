package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestInitConfig_IgnoresCurrentDirectory(t *testing.T) {
	// Create a wildgecu.yaml in a temp dir that should NOT be picked up.
	tmpDir := t.TempDir()

	cfgContent := []byte("gemini_api_key: from-local-dir\nmodel: local-model\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "wildgecu.yaml"), cfgContent, 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Reset viper and global state, then run initConfig.
	viper.Reset()
	cfgFile = ""
	initConfig()

	// The local directory config should NOT have been loaded.
	if got := viper.GetString("gemini_api_key"); got == "from-local-dir" {
		t.Error("initConfig loaded config from current directory; expected it to only use global home")
	}

	if got := viper.GetString("model"); got == "local-model" {
		t.Error("initConfig loaded model from current directory; expected it to only use global home")
	}
}

func TestInitConfig_LoadsFromGlobalHome(t *testing.T) {
	// Override HOME so GlobalHome() creates config in a temp dir.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	globalDir := filepath.Join(tmpHome, ".wildgecu")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgContent := []byte("gemini_api_key: from-global-home\nmodel: global-model\n")
	if err := os.WriteFile(filepath.Join(globalDir, "wildgecu.yaml"), cfgContent, 0o644); err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	cfgFile = ""
	initConfig()

	if got := viper.GetString("gemini_api_key"); got != "from-global-home" {
		t.Errorf("gemini_api_key = %q, want %q", got, "from-global-home")
	}

	if got := viper.GetString("model"); got != "global-model" {
		t.Errorf("model = %q, want %q", got, "global-model")
	}
}
