package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FSFile is a File backed by a single filesystem path.
type FSFile struct {
	path string
}

// NewFSFile creates a File that operates on the given filesystem path.
func NewFSFile(path string) File {
	return &FSFile{path: path}
}

func (f *FSFile) Get() (string, error) {
	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *FSFile) Write(content string) error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("file: create dir: %w", err)
	}
	return os.WriteFile(f.path, []byte(content), 0o644)
}

func (f *FSFile) Replace(old, replacement string) error {
	data, err := os.ReadFile(f.path) //nolint:gosec // path is not user-controlled
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	content := string(data)
	count := strings.Count(content, old)
	if count == 0 {
		return ErrOldNotFound
	}
	if count > 1 {
		return ErrNotUnique
	}

	replaced := strings.Replace(content, old, replacement, 1)
	return os.WriteFile(f.path, []byte(replaced), 0o644) //nolint:gosec // path is not user-controlled
}
