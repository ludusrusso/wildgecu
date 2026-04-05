package command

import (
	"context"
	"testing"

	"wildgecu/pkg/skill"
)

// stubCommand is a simple Command implementation for testing.
type stubCommand struct {
	name string
	desc string
}

func (c *stubCommand) Name() string        { return c.name }
func (c *stubCommand) Description() string { return c.desc }
func (c *stubCommand) Execute(_ context.Context, _ string) (string, error) {
	return "executed " + c.name, nil
}

func TestRegistryResolveBuiltin(t *testing.T) {
	r := NewRegistry("")
	r.Register(&stubCommand{name: "help", desc: "Show help"})

	cmd := r.Resolve("help")
	if cmd == nil {
		t.Fatal("expected to resolve built-in command 'help'")
	}
	if cmd.Name() != "help" {
		t.Errorf("expected name %q, got %q", "help", cmd.Name())
	}
}

func TestRegistryResolveUnknown(t *testing.T) {
	r := NewRegistry("")
	cmd := r.Resolve("nonexistent")
	if cmd != nil {
		t.Errorf("expected nil for unknown command, got %v", cmd)
	}
}

func TestRegistryResolveSkillFallback(t *testing.T) {
	dir := t.TempDir()
	writeTestSkill(t, dir, "---\nname: deploy\ndescription: Deploy the app\n---\nDeploy instructions")

	r := NewRegistry(dir)
	cmd := r.Resolve("deploy")
	if cmd == nil {
		t.Fatal("expected to resolve skill command 'deploy'")
	}
	if cmd.Name() != "deploy" {
		t.Errorf("expected name %q, got %q", "deploy", cmd.Name())
	}
	if cmd.Description() != "Deploy the app" {
		t.Errorf("expected description %q, got %q", "Deploy the app", cmd.Description())
	}
}

func TestRegistryBuiltinPriorityOverSkill(t *testing.T) {
	dir := t.TempDir()
	writeTestSkill(t, dir, "---\nname: help\ndescription: Skill help\n---\nSkill help content")

	r := NewRegistry(dir)
	r.Register(&stubCommand{name: "help", desc: "Built-in help"})

	cmd := r.Resolve("help")
	if cmd == nil {
		t.Fatal("expected to resolve command 'help'")
	}
	if cmd.Description() != "Built-in help" {
		t.Errorf("expected built-in to win, got description %q", cmd.Description())
	}
}

func TestRegistryListMerging(t *testing.T) {
	dir := t.TempDir()
	writeTestSkill(t, dir, "---\nname: deploy\ndescription: Deploy the app\n---\nDeploy instructions")
	writeTestSkill(t, dir, "---\nname: help\ndescription: Skill help\n---\nOverridden by built-in")

	r := NewRegistry(dir)
	r.Register(&stubCommand{name: "help", desc: "Built-in help"})

	entries := r.List()

	// Should have 2 entries: help (built-in) and deploy (skill).
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}

	found := map[string]string{}
	for _, e := range entries {
		found[e.Name] = e.Description
	}

	if found["help"] != "Built-in help" {
		t.Errorf("expected 'help' from built-in, got %q", found["help"])
	}
	if found["deploy"] != "Deploy the app" {
		t.Errorf("expected 'deploy' from skill, got %q", found["deploy"])
	}
}

func TestRegistryListEmpty(t *testing.T) {
	r := NewRegistry("")
	entries := r.List()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestSkillCommandImplementsSkillRunner(t *testing.T) {
	dir := t.TempDir()
	writeTestSkill(t, dir, "---\nname: review\ndescription: Code review\n---\nReview the code carefully")

	r := NewRegistry(dir)
	cmd := r.Resolve("review")
	if cmd == nil {
		t.Fatal("expected to resolve skill command")
	}

	runner, ok := cmd.(SkillRunner)
	if !ok {
		t.Fatal("expected skill command to implement SkillRunner")
	}

	if got := runner.SkillContent(); got != "Review the code carefully" {
		t.Errorf("SkillContent() = %q, want %q", got, "Review the code carefully")
	}
}

func TestBuiltinCommandNotSkillRunner(t *testing.T) {
	r := NewRegistry("")
	r.Register(&stubCommand{name: "help", desc: "Show help"})

	cmd := r.Resolve("help")
	if _, ok := cmd.(SkillRunner); ok {
		t.Error("expected built-in command NOT to implement SkillRunner")
	}
}

// writeTestSkill creates a skill directory with a SKILL.md file for testing.
func writeTestSkill(t *testing.T, dir, data string) {
	t.Helper()
	s, err := skill.Parse([]byte(data))
	if err != nil {
		t.Fatalf("parse test skill: %v", err)
	}
	if err := skill.Save(dir, s); err != nil {
		t.Fatalf("save test skill: %v", err)
	}
}
