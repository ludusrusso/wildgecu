package home

import (
	"fmt"
	"os"
	"path/filepath"

	"wildgecu/x/file"
)

// Home represents a wildgecu home directory with typed accessors.
type Home struct {
	dir string
}

// New creates a Home rooted at dir, creating the directory if needed.
func New(dir string) (*Home, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("home: create dir: %w", err)
	}
	return &Home{dir: dir}, nil
}

// Dir returns the root directory path.
func (h *Home) Dir() string {
	return h.dir
}

// Soul returns a File handle for SOUL.md.
func (h *Home) Soul() file.File {
	return file.NewFSFile(filepath.Join(h.dir, "SOUL.md"))
}

// Memory returns a File handle for MEMORY.md.
func (h *Home) Memory() file.File {
	return file.NewFSFile(filepath.Join(h.dir, "MEMORY.md"))
}

// User returns a File handle for USER.md.
func (h *Home) User() file.File {
	return file.NewFSFile(filepath.Join(h.dir, "USER.md"))
}

// SkillsDir returns the path to the skills subdirectory.
func (h *Home) SkillsDir() string {
	return filepath.Join(h.dir, "skills")
}

// CronsDir returns the path to the crons subdirectory.
func (h *Home) CronsDir() string {
	return filepath.Join(h.dir, "crons")
}

// CronResultsDir returns the path to the cron-results subdirectory.
func (h *Home) CronResultsDir() string {
	return filepath.Join(h.dir, "cron-results")
}
