package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// pathsFixture builds a workspace with files at known relative paths and
// applies a deterministic mtime ladder so mtime-desc sort is testable.
//
//	root/
//	├── README.md
//	├── a.go
//	├── pkg/agent.go
//	├── pkg/nested/deep.go
//	├── pkg/nested/agent_test.go
//	├── .git/HEAD              (skipped)
//	└── node_modules/x.js      (skipped)
//
// Files are touched in the order listed below so that touchOrder[0] has the
// oldest mtime and touchOrder[len-1] is the newest.
func pathsFixture(t *testing.T) (root string, touchOrder []string) {
	t.Helper()
	root = t.TempDir()

	files := []string{
		"README.md",
		"a.go",
		"pkg/agent.go",
		"pkg/nested/deep.go",
		"pkg/nested/agent_test.go",
		".git/HEAD",
		"node_modules/x.js",
	}
	for _, rel := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("// "+rel+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	touchOrder = []string{
		"a.go",
		"pkg/agent.go",
		"README.md",
		"pkg/nested/deep.go",
		"pkg/nested/agent_test.go",
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, rel := range touchOrder {
		full := filepath.Join(root, filepath.FromSlash(rel))
		mt := base.Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(full, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	return root, touchOrder
}

func TestPaths(t *testing.T) {
	t.Run("matches all go files via doublestar", func(t *testing.T) {
		root, _ := pathsFixture(t)
		got, err := Paths(context.Background(), root, "**/*.go", PathOptions{Sort: SortLex})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			"a.go",
			"pkg/agent.go",
			"pkg/nested/agent_test.go",
			"pkg/nested/deep.go",
		}
		if got.Total != len(want) {
			t.Fatalf("total = %d, want %d (paths=%v)", got.Total, len(want), got.Paths)
		}
		for i, p := range got.Paths {
			if p != want[i] {
				t.Errorf("paths[%d] = %q, want %q", i, p, want[i])
			}
		}
	})

	t.Run("single-star scoped to package directory", func(t *testing.T) {
		root, _ := pathsFixture(t)
		got, err := Paths(context.Background(), root, "pkg/*.go", PathOptions{Sort: SortLex})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"pkg/agent.go"}
		if len(got.Paths) != len(want) {
			t.Fatalf("paths len = %d, want %d (paths=%v)", len(got.Paths), len(want), got.Paths)
		}
		if got.Paths[0] != want[0] {
			t.Errorf("paths[0] = %q, want %q", got.Paths[0], want[0])
		}
	})

	t.Run("test-file pattern", func(t *testing.T) {
		root, _ := pathsFixture(t)
		got, err := Paths(context.Background(), root, "**/*_test.go", PathOptions{Sort: SortLex})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Paths) != 1 || got.Paths[0] != "pkg/nested/agent_test.go" {
			t.Fatalf("paths = %v, want [pkg/nested/agent_test.go]", got.Paths)
		}
	})

	t.Run("default sort is mtime descending", func(t *testing.T) {
		root, touchOrder := pathsFixture(t)
		got, err := Paths(context.Background(), root, "**/*.go", PathOptions{})
		if err != nil {
			t.Fatal(err)
		}
		// touchOrder lists oldest-first; the .go subset reversed should equal
		// the mtime-desc ordering.
		var goOrder []string
		for i := len(touchOrder) - 1; i >= 0; i-- {
			if strings.HasSuffix(touchOrder[i], ".go") {
				goOrder = append(goOrder, touchOrder[i])
			}
		}
		if len(got.Paths) != len(goOrder) {
			t.Fatalf("paths len = %d, want %d (paths=%v)", len(got.Paths), len(goOrder), got.Paths)
		}
		for i, p := range got.Paths {
			if p != goOrder[i] {
				t.Errorf("paths[%d] = %q, want %q (full=%v)", i, p, goOrder[i], got.Paths)
			}
		}
	})

	t.Run("max results truncates with total preserved", func(t *testing.T) {
		root, _ := pathsFixture(t)
		got, err := Paths(context.Background(), root, "**/*.go", PathOptions{Sort: SortLex, MaxResults: 2})
		if err != nil {
			t.Fatal(err)
		}
		if got.Total != 4 {
			t.Fatalf("total = %d, want 4", got.Total)
		}
		if len(got.Paths) != 2 {
			t.Fatalf("paths len = %d, want 2", len(got.Paths))
		}
		if !got.Truncated {
			t.Fatal("truncated should be true")
		}
	})

	t.Run("default skip dirs are skipped", func(t *testing.T) {
		root, _ := pathsFixture(t)
		got, err := Paths(context.Background(), root, "**/*", PathOptions{Sort: SortLex})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range got.Paths {
			if strings.HasPrefix(p, ".git/") || strings.HasPrefix(p, "node_modules/") {
				t.Fatalf("skip dir leaked into results: %q", p)
			}
		}
	})

	t.Run("path scope under root", func(t *testing.T) {
		root, _ := pathsFixture(t)
		got, err := Paths(context.Background(), root, "**/*.go", PathOptions{Path: "pkg", Sort: SortLex})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range got.Paths {
			if !strings.HasPrefix(p, "pkg/") {
				t.Errorf("path %q escaped pkg/ scope", p)
			}
		}
		if len(got.Paths) == 0 {
			t.Fatal("expected matches under pkg/")
		}
	})

	t.Run("path outside root rejected", func(t *testing.T) {
		root, _ := pathsFixture(t)
		_, err := Paths(context.Background(), root, "**/*.go", PathOptions{Path: "../etc"})
		if err == nil {
			t.Fatal("expected error for path escaping root")
		}
	})

	t.Run("absolute path outside root rejected", func(t *testing.T) {
		root, _ := pathsFixture(t)
		_, err := Paths(context.Background(), root, "**/*.go", PathOptions{Path: "/tmp"})
		if err == nil {
			t.Fatal("expected error for absolute path outside root")
		}
	})

	t.Run("invalid sort returns error", func(t *testing.T) {
		root, _ := pathsFixture(t)
		_, err := Paths(context.Background(), root, "**/*.go", PathOptions{Sort: "weird"})
		if err == nil {
			t.Fatal("expected error for invalid sort")
		}
	})

	t.Run("empty pattern returns error", func(t *testing.T) {
		root, _ := pathsFixture(t)
		_, err := Paths(context.Background(), root, "", PathOptions{})
		if err == nil {
			t.Fatal("expected error for empty pattern")
		}
	})

	t.Run("invalid pattern returns error", func(t *testing.T) {
		root, _ := pathsFixture(t)
		_, err := Paths(context.Background(), root, "[unclosed", PathOptions{})
		if err == nil {
			t.Fatal("expected error for invalid pattern")
		}
	})
}
