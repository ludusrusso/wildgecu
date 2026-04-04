package file

import (
	"path/filepath"
	"testing"
)

func TestFSFile(t *testing.T) {
	dir := t.TempDir()
	f := NewFSFile(filepath.Join(dir, "test.txt"))
	RunFileSpec(t, f)
}

func TestFSFile_ReplaceMissing(t *testing.T) {
	dir := t.TempDir()
	f := NewFSFile(filepath.Join(dir, "nonexistent.txt"))
	RunFileSpecReplaceMissing(t, f)
}
