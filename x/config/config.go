package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DirName is the name of the wildgecu configuration directory.
const DirName = ".wildgecu"

var globalHomeOverride string

// SetGlobalHome overrides the default home directory.
// The path must be absolute. Call this before any other config function.
func SetGlobalHome(path string) {
	globalHomeOverride = path
}

// GlobalHome returns the path to the wildgecu home directory, creating it if
// necessary. By default this is ~/.wildgecu/; use SetGlobalHome to override.
func GlobalHome() (string, error) {
	var dir string
	if globalHomeOverride != "" {
		dir = globalHomeOverride
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home dir: %w", err)
		}
		dir = filepath.Join(home, DirName)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create wildgecu home: %w", err)
	}
	return dir, nil
}

// GlobalFilePath returns the path to ~/.wildgecu/<filename>.
func GlobalFilePath(filename string) (string, error) {
	dir, err := GlobalHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filename), nil
}

// ProjectDir returns the path to <baseDir>/.wildgecu/, creating it if necessary.
func ProjectDir(baseDir string) (string, error) {
	dir := filepath.Join(baseDir, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create project wildgecu dir: %w", err)
	}
	return dir, nil
}

// ProjectFilePath returns the path to <baseDir>/.wildgecu/<filename>.
func ProjectFilePath(baseDir, filename string) (string, error) {
	dir := filepath.Join(baseDir, DirName)
	return filepath.Join(dir, filename), nil
}

const defaultConfig = `# wildgecu configuration
# provider: "gemini", "openai", or "ollama"
provider: "gemini"
model: "gemini-3-flash-preview"

# Provider API keys (set the one matching your provider)
gemini_api_key: ""
openai_api_key: ""

# Ollama settings (no API key required)
# ollama_base_url: "http://localhost:11434/v1"

# base_folder: "/path/to/project"
`

// EnsureConfigFile creates a default wildgecu.yaml in ~/.wildgecu/ if no config
// file is currently loaded. Returns the path to the config file and whether
// it was newly created.
func EnsureConfigFile(viperConfigUsed string) (string, bool, error) {
	if viperConfigUsed != "" {
		return viperConfigUsed, false, nil
	}

	configPath, err := GlobalFilePath("wildgecu.yaml")
	if err != nil {
		return "", false, fmt.Errorf("resolve config path: %w", err)
	}

	if _, err := os.Stat(configPath); err == nil {
		return configPath, false, nil
	}

	if err := os.WriteFile(configPath, []byte(defaultConfig), 0o644); err != nil {
		return "", false, fmt.Errorf("create config file: %w", err)
	}
	return configPath, true, nil
}
