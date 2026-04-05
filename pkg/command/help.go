package command

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// HelpCommand is a built-in command that lists all available commands.
type HelpCommand struct {
	registry *Registry
}

// NewHelpCommand creates a /help command bound to the given registry.
func NewHelpCommand(r *Registry) *HelpCommand {
	return &HelpCommand{registry: r}
}

func (c *HelpCommand) Name() string        { return "help" }
func (c *HelpCommand) Description() string { return "Show all available commands" }

func (c *HelpCommand) Execute(_ context.Context, _ string) (string, error) {
	entries := c.registry.List()
	if len(entries) == 0 {
		return "No commands available.", nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	var b strings.Builder
	b.WriteString("Available commands:\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "  /%s — %s\n", e.Name, e.Description)
	}
	return b.String(), nil
}
