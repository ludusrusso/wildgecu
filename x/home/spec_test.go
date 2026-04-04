package home

import (
	"bytes"
	"errors"
	"testing"
)

func RunHomeSpec(t *testing.T, h Home) {
	t.Run("Get missing key returns ErrNotFound", func(t *testing.T) {
		_, err := h.Get("nonexistent")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Upsert then Get returns correct data", func(t *testing.T) {
		data := []byte("hello world")
		if err := h.Upsert("file1.txt", data); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		got, err := h.Get("file1.txt")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("expected %q, got %q", data, got)
		}
	})

	t.Run("Upsert overwrite replaces first", func(t *testing.T) {
		if err := h.Upsert("file2.txt", []byte("first")); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		if err := h.Upsert("file2.txt", []byte("second")); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		got, err := h.Get("file2.txt")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if string(got) != "second" {
			t.Fatalf("expected %q, got %q", "second", got)
		}
	})

	t.Run("Search no matches returns empty slice", func(t *testing.T) {
		matches, err := h.Search("*.xyz")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(matches) != 0 {
			t.Fatalf("expected empty slice, got %v", matches)
		}
	})

	t.Run("Search with matches returns matching filenames", func(t *testing.T) {
		if err := h.Upsert("notes.md", []byte("# Notes")); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		matches, err := h.Search("*.md")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if len(matches) != 1 || matches[0] != "notes.md" {
			t.Fatalf("expected [notes.md], got %v", matches)
		}
	})

	t.Run("Delete missing key returns ErrNotFound", func(t *testing.T) {
		err := h.Delete("nonexistent")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Delete existing key removes it", func(t *testing.T) {
		if err := h.Upsert("to-delete.txt", []byte("bye")); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		if err := h.Delete("to-delete.txt"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		_, err := h.Get("to-delete.txt")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("Delete is idempotent after first call", func(t *testing.T) {
		if err := h.Upsert("once.txt", []byte("data")); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		if err := h.Delete("once.txt"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		err := h.Delete("once.txt")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound on second delete, got %v", err)
		}
	})

	t.Run("Search multiple matches returns all", func(t *testing.T) {
		if err := h.Upsert("a.txt", []byte("a")); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		if err := h.Upsert("b.txt", []byte("b")); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		matches, err := h.Search("*.txt")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		// file1.txt, file2.txt, a.txt, b.txt from earlier subtests
		if len(matches) < 2 {
			t.Fatalf("expected at least 2 matches, got %v", matches)
		}
		found := map[string]bool{}
		for _, m := range matches {
			found[m] = true
		}
		if !found["a.txt"] || !found["b.txt"] {
			t.Fatalf("expected a.txt and b.txt in results, got %v", matches)
		}
	})
}
