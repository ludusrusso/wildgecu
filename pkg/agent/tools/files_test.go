package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ludusrusso/wildgecu/pkg/provider/tool"
)

// helper to execute a tool and unmarshal the JSON result into dst.
func execTool(t *testing.T, tl tool.Tool, args map[string]any, dst any) {
	t.Helper()
	result, err := tl.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("execute %s: %v", tl.Definition().Name, err)
	}
	if err := json.Unmarshal([]byte(result), dst); err != nil {
		t.Fatalf("unmarshal result: %v\nraw: %s", err, result)
	}
}

func TestFileTools(t *testing.T) {
	tools := FileTools("/tmp")
	if len(tools) != 4 {
		t.Fatalf("expected 4 file tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tl := range tools {
		names[tl.Definition().Name] = true
	}
	for _, want := range []string{"list_files", "read_file", "write_file", "update_file"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestListFiles(t *testing.T) {
	t.Run("default dir", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)
		os.Mkdir(filepath.Join(dir, "sub"), 0o755)

		tl := newListFilesTool(dir)
		var out listFilesOutput
		execTool(t, tl, map[string]any{}, &out)

		if out.Path != dir {
			t.Fatalf("path = %q, want %q", out.Path, dir)
		}
		if len(out.Entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(out.Entries))
		}
	})

	t.Run("sub path", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "sub")
		os.Mkdir(sub, 0o755)
		os.WriteFile(filepath.Join(sub, "b.txt"), []byte("world"), 0o644)

		tl := newListFilesTool(dir)
		var out listFilesOutput
		execTool(t, tl, map[string]any{"path": "sub"}, &out)

		if len(out.Entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(out.Entries))
		}
		if out.Entries[0].Name != "b.txt" {
			t.Fatalf("entry name = %q", out.Entries[0].Name)
		}
	})

	t.Run("glob pattern", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.go"), []byte("go"), 0o644)
		os.WriteFile(filepath.Join(dir, "b.txt"), []byte("txt"), 0o644)
		os.WriteFile(filepath.Join(dir, "c.go"), []byte("go2"), 0o644)

		tl := newListFilesTool(dir)
		var out listFilesOutput
		execTool(t, tl, map[string]any{"pattern": "*.go"}, &out)

		if len(out.Entries) != 2 {
			t.Fatalf("expected 2 .go entries, got %d", len(out.Entries))
		}
	})

	t.Run("nonexistent dir", func(t *testing.T) {
		tl := newListFilesTool("/nonexistent/path")
		_, err := tl.Execute(context.Background(), map[string]any{})
		if err == nil {
			t.Fatal("expected error for nonexistent dir")
		}
	})
}

func TestReadFile(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "test.txt"), []byte("line1\nline2\nline3"), 0o644)

		tl := newReadFileTool(dir)
		var out readFileOutput
		execTool(t, tl, map[string]any{"path": "test.txt"}, &out)

		if out.TotalLines != 3 {
			t.Fatalf("total_lines = %d, want 3", out.TotalLines)
		}
		if out.Path != filepath.Join(dir, "test.txt") {
			t.Fatalf("path = %q", out.Path)
		}
	})

	t.Run("offset and limit", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\nb\nc\nd\ne"), 0o644)

		tl := newReadFileTool(dir)
		var out readFileOutput
		execTool(t, tl, map[string]any{"path": "f.txt", "offset": 2, "limit": 2}, &out)

		if out.TotalLines != 5 {
			t.Fatalf("total_lines = %d, want 5", out.TotalLines)
		}
		want := "2\tb\n3\tc\n"
		if out.Content != want {
			t.Fatalf("content = %q, want %q", out.Content, want)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		tl := newReadFileTool(t.TempDir())
		_, err := tl.Execute(context.Background(), map[string]any{"path": "nope.txt"})
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})
}

func TestWriteFile(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		dir := t.TempDir()
		tl := newWriteFileTool(dir)

		var out writeFileOutput
		execTool(t, tl, map[string]any{"path": "new.txt", "content": "hello world"}, &out)

		if out.Bytes != 11 {
			t.Fatalf("bytes = %d, want 11", out.Bytes)
		}
		data, err := os.ReadFile(filepath.Join(dir, "new.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "hello world" {
			t.Fatalf("file content = %q", string(data))
		}
	})

	t.Run("create parent dirs", func(t *testing.T) {
		dir := t.TempDir()
		tl := newWriteFileTool(dir)

		var out writeFileOutput
		execTool(t, tl, map[string]any{
			"path":    "a/b/c.txt",
			"content": "nested",
			"create":  true,
		}, &out)

		data, err := os.ReadFile(filepath.Join(dir, "a", "b", "c.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "nested" {
			t.Fatalf("file content = %q", string(data))
		}
	})

	t.Run("overwrites existing", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "f.txt"), []byte("old"), 0o644)

		tl := newWriteFileTool(dir)
		var out writeFileOutput
		execTool(t, tl, map[string]any{"path": "f.txt", "content": "new"}, &out)

		data, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
		if string(data) != "new" {
			t.Fatalf("file content = %q, want new", string(data))
		}
	})
}

func TestUpdateFile(t *testing.T) {
	t.Run("basic replace", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello world"), 0o644)

		tl := newUpdateFileTool(dir)
		var out updateFileOutput
		execTool(t, tl, map[string]any{
			"path":       "f.txt",
			"old_string": "world",
			"new_string": "gopher",
		}, &out)

		data, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
		if string(data) != "hello gopher" {
			t.Fatalf("file content = %q", string(data))
		}
	})

	t.Run("old string not found", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello"), 0o644)

		tl := newUpdateFileTool(dir)
		_, err := tl.Execute(context.Background(), map[string]any{
			"path":       "f.txt",
			"old_string": "missing",
			"new_string": "x",
		})
		if err == nil {
			t.Fatal("expected error for missing old_string")
		}
	})

	t.Run("duplicate old string", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "f.txt"), []byte("aa bb aa"), 0o644)

		tl := newUpdateFileTool(dir)
		_, err := tl.Execute(context.Background(), map[string]any{
			"path":       "f.txt",
			"old_string": "aa",
			"new_string": "cc",
		})
		if err == nil {
			t.Fatal("expected error for duplicate old_string")
		}

		data, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
		if string(data) != "aa bb aa" {
			t.Fatal("file should not have been modified")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		tl := newUpdateFileTool(t.TempDir())
		_, err := tl.Execute(context.Background(), map[string]any{
			"path":       "nope.txt",
			"old_string": "x",
			"new_string": "y",
		})
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})
}

func TestResolvePath(t *testing.T) {
	t.Run("relative path stays inside workDir", func(t *testing.T) {
		dir := t.TempDir()

		got, err := resolvePath(dir, "sub/file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(dir, "sub", "file.txt")
		if got != want {
			t.Fatalf("resolvePath = %q, want %q", got, want)
		}
	})

	t.Run("absolute path inside workDir is allowed", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a.txt")

		got, err := resolvePath(dir, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != target {
			t.Fatalf("resolvePath = %q, want %q", got, target)
		}
	})

	t.Run("absolute path outside workDir is rejected", func(t *testing.T) {
		dir := t.TempDir()

		if _, err := resolvePath(dir, "/etc/passwd"); err == nil {
			t.Fatal("expected error for absolute path outside workDir")
		}
	})

	t.Run("relative path escaping via dotdot is rejected", func(t *testing.T) {
		dir := t.TempDir()

		if _, err := resolvePath(dir, "../../../etc/passwd"); err == nil {
			t.Fatal("expected error for relative path escaping via ..")
		}
	})

	t.Run("symlink escaping workDir is rejected", func(t *testing.T) {
		work := t.TempDir()
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(work, "link")); err != nil {
			t.Skipf("cannot create symlink on this platform: %v", err)
		}

		if _, err := resolvePath(work, "link/secret.txt"); err == nil {
			t.Fatal("expected error for path escaping via symlink")
		}
	})

	t.Run("write path under missing parent inside workDir is allowed", func(t *testing.T) {
		dir := t.TempDir()

		got, err := resolvePath(dir, "newsub/new.txt")
		if err != nil {
			t.Fatalf("unexpected error for valid nested write path: %v", err)
		}
		want := filepath.Join(dir, "newsub", "new.txt")
		if got != want {
			t.Fatalf("resolvePath = %q, want %q", got, want)
		}
	})
}
