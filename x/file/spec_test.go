package file

import (
	"errors"
	"testing"
)

func RunFileSpec(t *testing.T, f File) {
	t.Run("Get missing file returns empty string and nil error", func(t *testing.T) {
		got, err := f.Get()
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}
	})

	t.Run("Write then Get round-trips correctly", func(t *testing.T) {
		content := "hello world"
		if err := f.Write(content); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		got, err := f.Get()
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got != content {
			t.Fatalf("expected %q, got %q", content, got)
		}
	})

	t.Run("Write overwrites existing content", func(t *testing.T) {
		if err := f.Write("first"); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if err := f.Write("second"); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		got, err := f.Get()
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got != "second" {
			t.Fatalf("expected %q, got %q", "second", got)
		}
	})

	t.Run("Replace with unique match succeeds", func(t *testing.T) {
		if err := f.Write("hello world"); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if err := f.Replace("world", "gopher"); err != nil {
			t.Fatalf("Replace failed: %v", err)
		}
		got, err := f.Get()
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got != "hello gopher" {
			t.Fatalf("expected %q, got %q", "hello gopher", got)
		}
	})

	t.Run("Replace with missing old string returns error", func(t *testing.T) {
		if err := f.Write("hello world"); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		err := f.Replace("nonexistent", "replacement")
		if !errors.Is(err, ErrOldNotFound) {
			t.Fatalf("expected ErrOldNotFound, got %v", err)
		}
	})

	t.Run("Replace with ambiguous old string returns error", func(t *testing.T) {
		if err := f.Write("aaa bbb aaa"); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		err := f.Replace("aaa", "ccc")
		if !errors.Is(err, ErrNotUnique) {
			t.Fatalf("expected ErrNotUnique, got %v", err)
		}
		// content should be unchanged
		got, err := f.Get()
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got != "aaa bbb aaa" {
			t.Fatalf("expected content unchanged, got %q", got)
		}
	})
}

// RunFileSpecReplaceMissing tests Replace on a file that does not exist.
// Callers must pass a File that points to a non-existent path.
func RunFileSpecReplaceMissing(t *testing.T, f File) {
	t.Run("Replace on missing file returns error", func(t *testing.T) {
		err := f.Replace("old", "new")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}
