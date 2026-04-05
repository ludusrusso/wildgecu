package tui

import (
	"strings"

	"wildgecu/pkg/daemon"
)

// Autocomplete filters and navigates a list of slash commands based on user input.
type Autocomplete struct {
	commands []daemon.CommandInfo
	matches  []daemon.CommandInfo
	selected int
	visible  bool
	lastPfx  string
}

// NewAutocomplete creates an Autocomplete with the given command list.
func NewAutocomplete(commands []daemon.CommandInfo) *Autocomplete {
	return &Autocomplete{commands: commands}
}

// Update recalculates the match list based on the current input text.
func (a *Autocomplete) Update(input string) {
	if !strings.HasPrefix(input, "/") || strings.Contains(input, " ") {
		a.visible = false
		a.matches = nil
		a.selected = 0
		a.lastPfx = ""
		return
	}

	prefix := strings.ToLower(input[1:]) // strip leading "/"
	if prefix != a.lastPfx {
		a.selected = 0
		a.lastPfx = prefix
	}

	a.matches = nil
	for _, cmd := range a.commands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), prefix) {
			a.matches = append(a.matches, cmd)
		}
	}

	a.visible = len(a.matches) > 0
	if a.selected >= len(a.matches) {
		a.selected = 0
	}
}

// Visible returns whether the dropdown should be shown.
func (a *Autocomplete) Visible() bool { return a.visible }

// Matches returns the current filtered command list.
func (a *Autocomplete) Matches() []daemon.CommandInfo { return a.matches }

// Selected returns the index of the highlighted item.
func (a *Autocomplete) Selected() int { return a.selected }

// MoveDown moves the selection down, wrapping around.
func (a *Autocomplete) MoveDown() {
	if len(a.matches) == 0 {
		return
	}
	a.selected = (a.selected + 1) % len(a.matches)
}

// MoveUp moves the selection up, wrapping around.
func (a *Autocomplete) MoveUp() {
	if len(a.matches) == 0 {
		return
	}
	a.selected = (a.selected - 1 + len(a.matches)) % len(a.matches)
}

// Complete returns the full command text for the selected item (e.g. "/help ").
// Returns empty string if the autocomplete is not visible.
func (a *Autocomplete) Complete() string {
	if !a.visible || len(a.matches) == 0 {
		return ""
	}
	return "/" + a.matches[a.selected].Name + " "
}
