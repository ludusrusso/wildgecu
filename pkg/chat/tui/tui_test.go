package tui

import (
	"testing"

	"github.com/ludusrusso/wildgecu/pkg/todo"
)

func TestStreamDoneMsgResetsTodosOnSessionChange(t *testing.T) {
	m := &Model{
		sessionID: "old-session",
		todos: []todo.Item{
			{ID: "1", Content: "foo"},
		},
		autocomplete: NewAutocomplete(nil),
		ready: true,
	}

	msg := streamDoneMsg{
		sessionID: "new-session",
	}

	newM, _ := m.Update(msg)
	updatedModel := newM.(Model) // Update actually takes a value receiver `func (m Model) Update` wait...

	if updatedModel.sessionID != "new-session" {
		t.Errorf("expected sessionID to be 'new-session', got %q", updatedModel.sessionID)
	}
	if len(updatedModel.todos) != 0 {
		t.Errorf("expected todos to be reset, but got %d items", len(updatedModel.todos))
	}
}

func TestStreamDoneMsgDoesNotResetTodosIfSessionSame(t *testing.T) {
	m := &Model{
		sessionID: "old-session",
		todos: []todo.Item{
			{ID: "1", Content: "foo"},
		},
		autocomplete: NewAutocomplete(nil),
		ready: true,
	}

	msg := streamDoneMsg{
		sessionID: "old-session",
	}

	newM, _ := m.Update(msg)
	updatedModel := newM.(Model)

	if len(updatedModel.todos) == 0 {
		t.Errorf("expected todos to not be reset")
	}
}
