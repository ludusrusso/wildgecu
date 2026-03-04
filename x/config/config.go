package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DirName is the name of the gonesis configuration directory.
const DirName = ".gonesis"

// GlobalHome returns the path to ~/.gonesis/, creating it if necessary.
func GlobalHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	dir := filepath.Join(home, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create gonesis home: %w", err)
	}
	return dir, nil
}

// GlobalFilePath returns the path to ~/.gonesis/<filename>.
func GlobalFilePath(filename string) (string, error) {
	dir, err := GlobalHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filename), nil
}

// ProjectDir returns the path to <baseDir>/.gonesis/, creating it if necessary.
func ProjectDir(baseDir string) (string, error) {
	dir := filepath.Join(baseDir, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create project gonesis dir: %w", err)
	}
	return dir, nil
}

// ProjectFilePath returns the path to <baseDir>/.gonesis/<filename>.
func ProjectFilePath(baseDir, filename string) (string, error) {
	dir := filepath.Join(baseDir, DirName)
	return filepath.Join(dir, filename), nil
}

const defaultConfig = `# gonesis configuration
gemini_api_key: ""
model: "gemini-3-flash-preview"
# base_folder: "/path/to/project"
`

// EnsureConfigFile creates a default gonesis.yaml in ~/.gonesis/ if no config
// file is currently loaded. Returns the path to the config file and whether
// it was newly created.
func EnsureConfigFile(viperConfigUsed string) (string, bool, error) {
	if viperConfigUsed != "" {
		return viperConfigUsed, false, nil
	}

	configPath, err := GlobalFilePath("gonesis.yaml")
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
