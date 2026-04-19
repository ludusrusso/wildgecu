package tui

import (
	"strings"
	"testing"

	"github.com/ludusrusso/wildgecu/pkg/todo"
)

func TestFormatToolCallLabel(t *testing.T) {
	t.Run("TodoCreateWithThreeItems", func(t *testing.T) {
		got := formatToolCallLabel("todo_create", "contents: [a b c]")
		want := "todo_create(3 items)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("TodoCreateWithOneItem", func(t *testing.T) {
		got := formatToolCallLabel("todo_create", "contents: [only]")
		want := "todo_create(1 item)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("TodoUpdateWithIdAndStatus", func(t *testing.T) {
		got := formatToolCallLabel("todo_update", "id: 2, status: completed")
		want := "todo_update(#2 → completed)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("TodoUpdateReversedOrder", func(t *testing.T) {
		got := formatToolCallLabel("todo_update", "status: in_progress, id: 7")
		want := "todo_update(#7 → in_progress)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("OtherToolUnchanged", func(t *testing.T) {
		got := formatToolCallLabel("read_file", "path: main.go")
		want := "read_file(path: main.go)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("OtherToolNoArgs", func(t *testing.T) {
		got := formatToolCallLabel("list_files", "")
		if got != "list_files" {
			t.Errorf("got %q, want %q", got, "list_files")
		}
	})

	t.Run("ToolWithNewlines", func(t *testing.T) {
		got := formatToolCallLabel("write_file", "path: foo.txt\ncontent: bar")
		want := "write_file(path: foo.txt\\ncontent: bar)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestRenderTodos(t *testing.T) {
	t.Run("EmptyListRendersNothing", func(t *testing.T) {
		m := &Model{}
		if got := m.renderTodos(); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
		if rows := m.todoRows(); rows != 0 {
			t.Errorf("todoRows() = %d, want 0", rows)
		}
	})

	t.Run("RendersOneLinePerItemWithStatusGlyphs", func(t *testing.T) {
		m := &Model{todos: []todo.Item{
			{ID: "1", Content: "first", Status: todo.StatusPending},
			{ID: "2", Content: "second", Status: todo.StatusInProgress},
			{ID: "3", Content: "third", Status: todo.StatusCompleted},
			{ID: "4", Content: "fourth", Status: todo.StatusCancelled},
		}}
		got := m.renderTodos()
		for _, want := range []string{"[ ] first", "[~] second", "[x] third", "[-] fourth"} {
			if !strings.Contains(got, want) {
				t.Errorf("rendered output missing %q\nfull output:\n%s", want, got)
			}
		}
		if rows := m.todoRows(); rows != 7 {
			t.Errorf("todoRows() = %d, want 7", rows)
		}
	})
}
