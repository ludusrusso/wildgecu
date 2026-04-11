package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads environment variables from a .env file in the given
// directory. If the file does not exist it returns nil. Existing environment
// variables take precedence over values defined in the file.
func LoadDotEnv(homeDir string) error {
	path := filepath.Join(homeDir, ".env")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := godotenv.Load(path); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}
	return nil
}
