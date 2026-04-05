package tui

import (
	"testing"

	"wildgecu/pkg/daemon"
)

var testCommands = []daemon.CommandInfo{
	{Name: "help", Description: "Show available commands"},
	{Name: "status", Description: "Show session info"},
	{Name: "clean", Description: "Reset current session"},
	{Name: "search", Description: "Search the web"},
}

func TestAutocomplete(t *testing.T) {
	t.Run("visible when input starts with slash and has no space", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/")
		if !ac.Visible() {
			t.Error("expected autocomplete to be visible for '/'")
		}
		ac.Update("/he")
		if !ac.Visible() {
			t.Error("expected autocomplete to be visible for '/he'")
		}
	})

	t.Run("hidden when input is empty", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("")
		if ac.Visible() {
			t.Error("expected autocomplete to be hidden for empty input")
		}
	})

	t.Run("hidden when input does not start with slash", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("hello")
		if ac.Visible() {
			t.Error("expected autocomplete to be hidden for 'hello'")
		}
	})

	t.Run("hidden when input contains a space", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/help arg")
		if ac.Visible() {
			t.Error("expected autocomplete to be hidden when input has a space")
		}
	})

	t.Run("filters commands by prefix case-insensitive", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/S")
		matches := ac.Matches()
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches for '/S', got %d: %+v", len(matches), matches)
		}
		// Should match "status" and "search"
		names := map[string]bool{}
		for _, m := range matches {
			names[m.Name] = true
		}
		if !names["status"] || !names["search"] {
			t.Errorf("expected status and search in matches, got %+v", matches)
		}
	})

	t.Run("slash alone shows all commands", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/")
		matches := ac.Matches()
		if len(matches) != len(testCommands) {
			t.Errorf("expected %d matches for '/', got %d", len(testCommands), len(matches))
		}
	})

	t.Run("no matches hides autocomplete", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/zzz")
		if ac.Visible() {
			t.Error("expected autocomplete to be hidden when no commands match")
		}
	})

	t.Run("selected index starts at zero", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/")
		if ac.Selected() != 0 {
			t.Errorf("expected selected index 0, got %d", ac.Selected())
		}
	})

	t.Run("move down increments selected", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/")
		ac.MoveDown()
		if ac.Selected() != 1 {
			t.Errorf("expected selected index 1, got %d", ac.Selected())
		}
	})

	t.Run("move down wraps around", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/")
		for range len(testCommands) {
			ac.MoveDown()
		}
		if ac.Selected() != 0 {
			t.Errorf("expected selected to wrap to 0, got %d", ac.Selected())
		}
	})

	t.Run("move up wraps to last", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/")
		ac.MoveUp()
		want := len(testCommands) - 1
		if ac.Selected() != want {
			t.Errorf("expected selected to wrap to %d, got %d", want, ac.Selected())
		}
	})

	t.Run("selection resets when filter changes", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/")
		ac.MoveDown()
		ac.MoveDown()
		ac.Update("/s")
		if ac.Selected() != 0 {
			t.Errorf("expected selected to reset to 0 after filter change, got %d", ac.Selected())
		}
	})

	t.Run("complete returns selected command with trailing space", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/")
		result := ac.Complete()
		if result == "" {
			t.Fatal("expected non-empty completion")
		}
		// First match with trailing space.
		expected := "/" + ac.Matches()[0].Name + " "
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	t.Run("complete with navigated selection", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("/")
		ac.MoveDown()
		result := ac.Complete()
		expected := "/" + ac.Matches()[1].Name + " "
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	t.Run("complete returns empty when not visible", func(t *testing.T) {
		ac := NewAutocomplete(testCommands)
		ac.Update("hello")
		if result := ac.Complete(); result != "" {
			t.Errorf("expected empty completion, got %q", result)
		}
	})
}
