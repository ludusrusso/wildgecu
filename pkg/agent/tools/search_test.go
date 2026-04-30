package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// grepFixture lays out a small workspace used by the grep tool tests.
//
//	root/
//	├── a.go        "package a\nfunc Foo() {}\n// TODO: refactor\n"
//	├── b.go        "package b\nfunc Bar() {}\n"
//	├── notes.md    "Read me\nTODO: write more\n"
//	└── pkg/
//	    └── deep.go "func Deep() {} // TODO\n"
func grepFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"a.go":        "package a\nfunc Foo() {}\n// TODO: refactor\n",
		"b.go":        "package b\nfunc Bar() {}\n",
		"notes.md":    "Read me\nTODO: write more\n",
		"pkg/deep.go": "func Deep() {} // TODO\n",
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestSearchTools(t *testing.T) {
	tls := SearchTools("/tmp", SearchConfig{})
	want := []string{"grep", "glob"}
	if len(tls) != len(want) {
		t.Fatalf("expected %d search tools, got %d", len(want), len(tls))
	}
	for i, name := range want {
		if tls[i].Definition().Name != name {
			t.Fatalf("tool[%d] name = %q, want %q", i, tls[i].Definition().Name, name)
		}
	}
}

func TestGrepTool(t *testing.T) {
	t.Run("schema lists pattern as required", func(t *testing.T) {
		tl := newGrepTool("/tmp", SearchConfig{})
		def := tl.Definition()
		if def.Name != "grep" {
			t.Fatalf("name = %q", def.Name)
		}
		params, _ := def.Parameters["properties"].(map[string]any)
		if _, ok := params["pattern"]; !ok {
			t.Fatal("schema missing pattern property")
		}
		req, _ := def.Parameters["required"].([]any)
		found := false
		for _, name := range req {
			if name == "pattern" {
				found = true
			}
		}
		if !found {
			t.Fatal("pattern should be required in schema")
		}
	})

	t.Run("content mode default", func(t *testing.T) {
		root := grepFixture(t)
		tl := newGrepTool(root, SearchConfig{})
		var out grepOutput
		execTool(t, tl, map[string]any{"pattern": "TODO"}, &out)

		if out.Mode != "content" {
			t.Fatalf("mode = %q, want content", out.Mode)
		}
		if out.Total != 3 {
			t.Fatalf("total = %d, want 3", out.Total)
		}
		if len(out.Matches) != 3 {
			t.Fatalf("matches len = %d, want 3", len(out.Matches))
		}
	})

	t.Run("files_with_matches mode", func(t *testing.T) {
		root := grepFixture(t)
		tl := newGrepTool(root, SearchConfig{})
		var out grepOutput
		execTool(t, tl, map[string]any{"pattern": "TODO", "output_mode": "files_with_matches"}, &out)

		if out.Mode != "files_with_matches" {
			t.Fatalf("mode = %q", out.Mode)
		}
		if len(out.Files) != 3 {
			t.Fatalf("files len = %d, want 3", len(out.Files))
		}
		// Lex sorted.
		want := []string{"a.go", "notes.md", "pkg/deep.go"}
		for i, p := range out.Files {
			if p != want[i] {
				t.Errorf("file[%d] = %q, want %q", i, p, want[i])
			}
		}
	})

	t.Run("count mode", func(t *testing.T) {
		root := grepFixture(t)
		tl := newGrepTool(root, SearchConfig{})
		var out grepOutput
		execTool(t, tl, map[string]any{"pattern": "func", "output_mode": "count"}, &out)

		if out.Mode != "count" {
			t.Fatalf("mode = %q", out.Mode)
		}
		if len(out.Counts) == 0 {
			t.Fatal("expected count rows")
		}
		// All file paths should be set; counts > 0.
		for _, row := range out.Counts {
			if row.Path == "" || row.Count <= 0 {
				t.Errorf("bad row: %+v", row)
			}
		}
	})

	t.Run("invalid output mode", func(t *testing.T) {
		root := grepFixture(t)
		tl := newGrepTool(root, SearchConfig{})
		_, err := tl.Execute(context.Background(), map[string]any{
			"pattern":     "TODO",
			"output_mode": "bogus",
		})
		if err == nil {
			t.Fatal("expected error for invalid output_mode")
		}
	})

	t.Run("head limit truncates with indicator", func(t *testing.T) {
		root := grepFixture(t)
		tl := newGrepTool(root, SearchConfig{})
		var out grepOutput
		execTool(t, tl, map[string]any{
			"pattern":    "TODO",
			"head_limit": 1,
		}, &out)

		if out.Total != 3 {
			t.Fatalf("total = %d, want 3", out.Total)
		}
		if !out.Truncated {
			t.Fatal("truncated should be true")
		}
		if out.Indicator == "" {
			t.Fatal("indicator should be populated when truncated")
		}
		if len(out.Matches) != 1 {
			t.Fatalf("matches len = %d, want 1", len(out.Matches))
		}
	})

	t.Run("path scope outside root rejected", func(t *testing.T) {
		root := grepFixture(t)
		tl := newGrepTool(root, SearchConfig{})
		_, err := tl.Execute(context.Background(), map[string]any{
			"pattern": "TODO",
			"path":    "../etc",
		})
		if err == nil {
			t.Fatal("expected error for path escaping root")
		}
	})

	t.Run("case insensitive flag", func(t *testing.T) {
		root := grepFixture(t)
		tl := newGrepTool(root, SearchConfig{})
		var out grepOutput
		execTool(t, tl, map[string]any{
			"pattern":          "todo",
			"case_insensitive": true,
		}, &out)

		if out.Total != 3 {
			t.Fatalf("total = %d, want 3 (case-insensitive)", out.Total)
		}
	})

	t.Run("config max results respected", func(t *testing.T) {
		root := grepFixture(t)
		tl := newGrepTool(root, SearchConfig{MaxResults: 1})
		var out grepOutput
		execTool(t, tl, map[string]any{"pattern": "TODO"}, &out)

		if !out.Truncated {
			t.Fatal("expected truncation due to MaxResults=1")
		}
		if len(out.Matches) != 1 {
			t.Fatalf("matches len = %d, want 1", len(out.Matches))
		}
	})
}

// globFixture lays out a small workspace whose Go files have a known mtime
// ladder so default mtime-desc ordering is testable.
func globFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []string{
		"a.go",
		"pkg/agent.go",
		"pkg/nested/deep.go",
		"README.md",
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
	// Apply mtimes oldest-first so reverse order is the mtime-desc result.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, rel := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		mt := base.Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(full, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestGlobTool(t *testing.T) {
	t.Run("schema lists pattern as required", func(t *testing.T) {
		tl := newGlobTool("/tmp")
		def := tl.Definition()
		if def.Name != "glob" {
			t.Fatalf("name = %q", def.Name)
		}
		params, _ := def.Parameters["properties"].(map[string]any)
		if _, ok := params["pattern"]; !ok {
			t.Fatal("schema missing pattern property")
		}
		req, _ := def.Parameters["required"].([]any)
		found := false
		for _, name := range req {
			if name == "pattern" {
				found = true
			}
		}
		if !found {
			t.Fatal("pattern should be required in schema")
		}
	})

	t.Run("default sort is mtime descending", func(t *testing.T) {
		root := globFixture(t)
		tl := newGlobTool(root)
		var out globOutput
		execTool(t, tl, map[string]any{"pattern": "**/*.go"}, &out)

		want := []string{"pkg/nested/deep.go", "pkg/agent.go", "a.go"}
		if len(out.Paths) != len(want) {
			t.Fatalf("paths len = %d, want %d (paths=%v)", len(out.Paths), len(want), out.Paths)
		}
		for i, p := range out.Paths {
			if p != want[i] {
				t.Errorf("paths[%d] = %q, want %q", i, p, want[i])
			}
		}
	})

	t.Run("lex sort", func(t *testing.T) {
		root := globFixture(t)
		tl := newGlobTool(root)
		var out globOutput
		execTool(t, tl, map[string]any{"pattern": "**/*.go", "sort": "lex"}, &out)

		want := []string{"a.go", "pkg/agent.go", "pkg/nested/deep.go"}
		for i, p := range out.Paths {
			if p != want[i] {
				t.Errorf("paths[%d] = %q, want %q", i, p, want[i])
			}
		}
	})

	t.Run("max results truncates with indicator", func(t *testing.T) {
		root := globFixture(t)
		tl := newGlobTool(root)
		var out globOutput
		execTool(t, tl, map[string]any{"pattern": "**/*.go", "sort": "lex", "max_results": 1}, &out)

		if out.Total != 3 {
			t.Fatalf("total = %d, want 3", out.Total)
		}
		if !out.Truncated {
			t.Fatal("truncated should be true")
		}
		if out.Indicator == "" {
			t.Fatal("indicator should be populated when truncated")
		}
		if len(out.Paths) != 1 {
			t.Fatalf("paths len = %d, want 1", len(out.Paths))
		}
	})

	t.Run("path scope outside root rejected", func(t *testing.T) {
		root := globFixture(t)
		tl := newGlobTool(root)
		_, err := tl.Execute(context.Background(), map[string]any{
			"pattern": "**/*.go",
			"path":    "../etc",
		})
		if err == nil {
			t.Fatal("expected error for path escaping root")
		}
	})

	t.Run("invalid sort rejected", func(t *testing.T) {
		root := globFixture(t)
		tl := newGlobTool(root)
		_, err := tl.Execute(context.Background(), map[string]any{
			"pattern": "**/*.go",
			"sort":    "weird",
		})
		if err == nil {
			t.Fatal("expected error for invalid sort")
		}
	})
}
