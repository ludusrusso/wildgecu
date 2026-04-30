package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture builds a small workspace under t.TempDir().
//
//	root/
//	├── a.go               "package a\nfunc Foo() {}\n// TODO: refactor\n"
//	├── b.go               "package b\nfunc Bar() {}\n"
//	├── notes.md           "Read me\nTODO: write more\n"
//	├── bin.dat            (binary, contains NUL)
//	├── .git/HEAD          "ref: refs/heads/main"  (must be skipped)
//	├── node_modules/x.js  "var TODO = 1"          (must be skipped)
//	└── pkg/
//	    ├── nested/
//	    │   └── deep.go    "func Deep() {} // TODO\n"
//	    └── other.go       "func Other() {}\n"
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	files := map[string]string{
		"a.go":                 "package a\nfunc Foo() {}\n// TODO: refactor\n",
		"b.go":                 "package b\nfunc Bar() {}\n",
		"notes.md":             "Read me\nTODO: write more\n",
		".git/HEAD":            "ref: refs/heads/main",
		"node_modules/x.js":    "var TODO = 1",
		"pkg/nested/deep.go":   "func Deep() {} // TODO\n",
		"pkg/other.go":         "func Other() {}\n",
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Binary file with embedded NUL.
	bin := []byte{0x89, 'P', 'N', 'G', 0x00, 'T', 'O', 'D', 'O'}
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), bin, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestContent(t *testing.T) {
	t.Run("basic match counts and order", func(t *testing.T) {
		root := fixture(t)
		got, err := Content(context.Background(), root, Options{Pattern: "TODO"})
		if err != nil {
			t.Fatal(err)
		}
		want := []Match{
			{Path: "a.go", Line: 3, Text: "// TODO: refactor"},
			{Path: "notes.md", Line: 2, Text: "TODO: write more"},
			{Path: "pkg/nested/deep.go", Line: 1, Text: "func Deep() {} // TODO"},
		}
		if got.Total != len(want) {
			t.Fatalf("total = %d, want %d (matches=%v)", got.Total, len(want), got.Matches)
		}
		if len(got.Matches) != len(want) {
			t.Fatalf("matches len = %d, want %d", len(got.Matches), len(want))
		}
		for i, m := range got.Matches {
			if m != want[i] {
				t.Errorf("match[%d] = %+v, want %+v", i, m, want[i])
			}
		}
	})

	t.Run("binary file skipped", func(t *testing.T) {
		root := fixture(t)
		got, err := Content(context.Background(), root, Options{Pattern: "TODO"})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range got.Matches {
			if m.Path == "bin.dat" {
				t.Fatalf("binary file should have been skipped, got match: %+v", m)
			}
		}
	})

	t.Run("default skip dirs are skipped", func(t *testing.T) {
		root := fixture(t)
		got, err := Content(context.Background(), root, Options{Pattern: "TODO"})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range got.Matches {
			if strings.HasPrefix(m.Path, ".git/") || strings.HasPrefix(m.Path, "node_modules/") {
				t.Fatalf("skip dir leaked into results: %+v", m)
			}
		}
	})

	t.Run("head limit truncates with total preserved", func(t *testing.T) {
		root := fixture(t)
		got, err := Content(context.Background(), root, Options{Pattern: "TODO", HeadLimit: 2})
		if err != nil {
			t.Fatal(err)
		}
		if got.Total != 3 {
			t.Fatalf("total = %d, want 3", got.Total)
		}
		if len(got.Matches) != 2 {
			t.Fatalf("matches len = %d, want 2", len(got.Matches))
		}
		if !got.Truncated {
			t.Fatal("truncated should be true")
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		root := fixture(t)
		got, err := Content(context.Background(), root, Options{Pattern: "todo", CaseInsensitive: true})
		if err != nil {
			t.Fatal(err)
		}
		if got.Total < 3 {
			t.Fatalf("expected >=3 matches case-insensitive, got %d", got.Total)
		}
	})

	t.Run("path scope under root", func(t *testing.T) {
		root := fixture(t)
		got, err := Content(context.Background(), root, Options{Pattern: "func", Path: "pkg"})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range got.Matches {
			if !strings.HasPrefix(m.Path, "pkg/") {
				t.Errorf("unexpected match outside pkg/: %+v", m)
			}
		}
		if got.Total < 2 {
			t.Fatalf("expected >=2 matches under pkg/, got %d", got.Total)
		}
	})

	t.Run("path outside root rejected", func(t *testing.T) {
		root := fixture(t)
		_, err := Content(context.Background(), root, Options{Pattern: "x", Path: "../etc"})
		if err == nil {
			t.Fatal("expected error for path escaping root")
		}
	})

	t.Run("absolute path outside root rejected", func(t *testing.T) {
		root := fixture(t)
		_, err := Content(context.Background(), root, Options{Pattern: "x", Path: "/tmp"})
		if err == nil {
			t.Fatal("expected error for absolute path outside root")
		}
	})

	t.Run("glob filename filter", func(t *testing.T) {
		root := fixture(t)
		got, err := Content(context.Background(), root, Options{Pattern: "TODO", Glob: "*.md"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Total != 1 {
			t.Fatalf("total = %d, want 1 (matches=%v)", got.Total, got.Matches)
		}
		if got.Matches[0].Path != "notes.md" {
			t.Fatalf("path = %q, want notes.md", got.Matches[0].Path)
		}
	})

	t.Run("deterministic ordering across calls", func(t *testing.T) {
		root := fixture(t)
		first, err := Content(context.Background(), root, Options{Pattern: "func"})
		if err != nil {
			t.Fatal(err)
		}
		second, err := Content(context.Background(), root, Options{Pattern: "func"})
		if err != nil {
			t.Fatal(err)
		}
		if len(first.Matches) != len(second.Matches) {
			t.Fatalf("len differs: %d vs %d", len(first.Matches), len(second.Matches))
		}
		for i := range first.Matches {
			if first.Matches[i] != second.Matches[i] {
				t.Errorf("match[%d] differs: %+v vs %+v", i, first.Matches[i], second.Matches[i])
			}
		}
	})

	t.Run("max file size skips large files", func(t *testing.T) {
		root := t.TempDir()
		large := strings.Repeat("needle\n", 1000)
		small := "needle\n"
		if err := os.WriteFile(filepath.Join(root, "large.txt"), []byte(large), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "small.txt"), []byte(small), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := Content(context.Background(), root, Options{
			Pattern:     "needle",
			MaxFileSize: 32, // small.txt fits, large.txt doesn't
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range got.Matches {
			if m.Path == "large.txt" {
				t.Fatalf("large.txt should have been skipped, got match: %+v", m)
			}
		}
		if got.Total == 0 {
			t.Fatal("expected small.txt match, got none")
		}
	})

	t.Run("invalid regex returns error", func(t *testing.T) {
		root := t.TempDir()
		_, err := Content(context.Background(), root, Options{Pattern: "([unclosed"})
		if err == nil {
			t.Fatal("expected error for invalid regex")
		}
	})

	t.Run("empty pattern returns error", func(t *testing.T) {
		root := t.TempDir()
		_, err := Content(context.Background(), root, Options{})
		if err == nil {
			t.Fatal("expected error for empty pattern")
		}
	})
}
