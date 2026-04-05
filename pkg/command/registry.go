package command

import (
	"context"

	"wildgecu/pkg/skill"
)

// Command is the interface for a slash command.
type Command interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args string) (string, error)
}

// Entry is a name+description pair returned by Registry.List.
type Entry struct {
	Name        string
	Description string
}

// Registry holds built-in commands and falls back to skills for resolution.
type Registry struct {
	builtins  map[string]Command
	skillsDir string
}

// NewRegistry creates a new command registry. skillsDir is the path to the
// skills directory used for fallback resolution; it may be empty to disable
// skill fallback.
func NewRegistry(skillsDir string) *Registry {
	return &Registry{
		builtins:  make(map[string]Command),
		skillsDir: skillsDir,
	}
}

// Register adds a built-in command to the registry.
func (r *Registry) Register(cmd Command) {
	r.builtins[cmd.Name()] = cmd
}

// Resolve looks up a command by name. Built-in commands take priority; if not
// found, it falls back to loading a skill with the same name.
func (r *Registry) Resolve(name string) Command {
	if cmd, ok := r.builtins[name]; ok {
		return cmd
	}
	if r.skillsDir == "" {
		return nil
	}
	s, err := skill.Load(r.skillsDir, name)
	if err != nil {
		return nil
	}
	return &skillCommand{skill: s}
}

// List returns all available commands (built-in + skills), with built-in
// descriptions taking priority over skills with the same name.
func (r *Registry) List() []Entry {
	seen := make(map[string]bool)
	var entries []Entry

	for _, cmd := range r.builtins {
		entries = append(entries, Entry{Name: cmd.Name(), Description: cmd.Description()})
		seen[cmd.Name()] = true
	}

	if r.skillsDir != "" {
		skills, _ := skill.LoadAll(r.skillsDir)
		for _, s := range skills {
			if seen[s.Name] {
				continue
			}
			entries = append(entries, Entry{Name: s.Name, Description: s.Description})
		}
	}

	return entries
}

// SkillRunner is implemented by commands backed by a skill that require
// an LLM turn with the skill content as system context.
type SkillRunner interface {
	SkillContent() string
}

// skillCommand adapts a skill.Skill to the Command interface.
type skillCommand struct {
	skill *skill.Skill
}

func (c *skillCommand) Name() string           { return c.skill.Name }
func (c *skillCommand) Description() string    { return c.skill.Description }
func (c *skillCommand) SkillContent() string   { return c.skill.Content }
func (c *skillCommand) Execute(_ context.Context, _ string) (string, error) {
	return c.skill.Content, nil
}
