package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	if len(tools) != 5 {
		t.Fatalf("expected 5 file tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tl := range tools {
		names[tl.Definition().Name] = true
	}
	for _, want := range []string{"list_files", "read_file", "write_file", "update_file", "multi_edit"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestMultiEditTools(t *testing.T) {
	tools := MultiEditTools("/tmp")
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if got := tools[0].Definition().Name; got != "multi_edit" {
		t.Errorf("expected multi_edit, got %q", got)
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

func TestMultiEdit(t *testing.T) {
	t.Run("schema marks path and edits required", func(t *testing.T) {
		tl := newMultiEditTool("/tmp")
		params := tl.Definition().Parameters
		props, _ := params["properties"].(map[string]any)
		if _, ok := props["path"]; !ok {
			t.Error("expected path in properties")
		}
		if _, ok := props["edits"]; !ok {
			t.Error("expected edits in properties")
		}
		required, _ := params["required"].([]any)
		got := map[string]bool{}
		for _, r := range required {
			got[r.(string)] = true
		}
		if !got["path"] || !got["edits"] {
			t.Errorf("expected required={path, edits}, got %v", required)
		}
	})

	t.Run("applies sequential edits", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		os.WriteFile(path, []byte("hello world"), 0o644)

		tl := newMultiEditTool(dir)
		var out multiEditOutput
		execTool(t, tl, map[string]any{
			"path": "f.txt",
			"edits": []any{
				map[string]any{"old_string": "hello", "new_string": "hi"},
				map[string]any{"old_string": "world", "new_string": "gopher"},
			},
		}, &out)

		if out.EditsApplied != 2 {
			t.Errorf("edits_applied = %d, want 2", out.EditsApplied)
		}
		data, _ := os.ReadFile(path)
		if string(data) != "hi gopher" {
			t.Errorf("file content = %q, want %q", string(data), "hi gopher")
		}
	})

	t.Run("later edit targets earlier edit's output", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		os.WriteFile(path, []byte("foo"), 0o644)

		tl := newMultiEditTool(dir)
		var out multiEditOutput
		execTool(t, tl, map[string]any{
			"path": "f.txt",
			"edits": []any{
				map[string]any{"old_string": "foo", "new_string": "bar"},
				map[string]any{"old_string": "bar", "new_string": "baz"},
			},
		}, &out)

		data, _ := os.ReadFile(path)
		if string(data) != "baz" {
			t.Errorf("file content = %q, want %q", string(data), "baz")
		}
	})

	t.Run("replace_all per edit", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		os.WriteFile(path, []byte("aa bb aa cc aa"), 0o644)

		tl := newMultiEditTool(dir)
		var out multiEditOutput
		execTool(t, tl, map[string]any{
			"path": "f.txt",
			"edits": []any{
				map[string]any{"old_string": "aa", "new_string": "X", "replace_all": true},
				map[string]any{"old_string": "cc", "new_string": "Y"},
			},
		}, &out)

		data, _ := os.ReadFile(path)
		if string(data) != "X bb X Y X" {
			t.Errorf("file content = %q, want %q", string(data), "X bb X Y X")
		}
	})

	t.Run("atomic failure leaves file unchanged", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		original := []byte("alpha beta gamma")
		os.WriteFile(path, original, 0o644)

		tl := newMultiEditTool(dir)
		_, err := tl.Execute(context.Background(), map[string]any{
			"path": "f.txt",
			"edits": []any{
				map[string]any{"old_string": "alpha", "new_string": "A"},
				map[string]any{"old_string": "missing", "new_string": "Z"},
			},
		})
		if err == nil {
			t.Fatal("expected error for missing old_string")
		}
		if !strings.Contains(err.Error(), "edit 1") {
			t.Errorf("error should reference edit 1, got %v", err)
		}

		data, _ := os.ReadFile(path)
		if !bytes.Equal(data, original) {
			t.Errorf("file should be unchanged, got %q", string(data))
		}
	})

	t.Run("non-unique old_string without replace_all fails", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		original := []byte("aa bb aa")
		os.WriteFile(path, original, 0o644)

		tl := newMultiEditTool(dir)
		_, err := tl.Execute(context.Background(), map[string]any{
			"path": "f.txt",
			"edits": []any{
				map[string]any{"old_string": "aa", "new_string": "X"},
			},
		})
		if err == nil {
			t.Fatal("expected error for duplicate old_string")
		}
		if !strings.Contains(err.Error(), "edit 0") {
			t.Errorf("error should reference edit 0, got %v", err)
		}

		data, _ := os.ReadFile(path)
		if !bytes.Equal(data, original) {
			t.Errorf("file should be unchanged, got %q", string(data))
		}
	})

	t.Run("uniqueness checked against in-progress buffer", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		// Initially "x" appears once. After first edit replacing "y" with
		// "x", the buffer has two "x" — the second edit (without replace_all)
		// must fail because uniqueness is computed against the current buffer.
		os.WriteFile(path, []byte("x y"), 0o644)

		tl := newMultiEditTool(dir)
		_, err := tl.Execute(context.Background(), map[string]any{
			"path": "f.txt",
			"edits": []any{
				map[string]any{"old_string": "y", "new_string": "x"},
				map[string]any{"old_string": "x", "new_string": "Z"},
			},
		})
		if err == nil {
			t.Fatal("expected error: second edit should see two occurrences of x")
		}

		data, _ := os.ReadFile(path)
		if string(data) != "x y" {
			t.Errorf("file should be unchanged, got %q", string(data))
		}
	})

	t.Run("empty edits rejected", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		os.WriteFile(path, []byte("hello"), 0o644)

		tl := newMultiEditTool(dir)
		_, err := tl.Execute(context.Background(), map[string]any{
			"path":  "f.txt",
			"edits": []any{},
		})
		if err == nil {
			t.Fatal("expected error for empty edits")
		}
	})

	t.Run("identical old_string and new_string rejected", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		original := []byte("hello")
		os.WriteFile(path, original, 0o644)

		tl := newMultiEditTool(dir)
		_, err := tl.Execute(context.Background(), map[string]any{
			"path": "f.txt",
			"edits": []any{
				map[string]any{"old_string": "hello", "new_string": "hello"},
			},
		})
		if err == nil {
			t.Fatal("expected error for identical old/new")
		}

		data, _ := os.ReadFile(path)
		if !bytes.Equal(data, original) {
			t.Errorf("file should be unchanged, got %q", string(data))
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		tl := newMultiEditTool(t.TempDir())
		_, err := tl.Execute(context.Background(), map[string]any{
			"path": "nope.txt",
			"edits": []any{
				map[string]any{"old_string": "x", "new_string": "y"},
			},
		})
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestResolvePath(t *testing.T) {
	t.Run("relative", func(t *testing.T) {
		got := resolvePath("/work", "sub/file.txt")
		want := "/work/sub/file.txt"
		if got != want {
			t.Fatalf("resolvePath = %q, want %q", got, want)
		}
	})

	t.Run("absolute", func(t *testing.T) {
		got := resolvePath("/work", "/tmp/file.txt")
		if got != "/tmp/file.txt" {
			t.Fatalf("resolvePath = %q, want /tmp/file.txt", got)
		}
	})
}
