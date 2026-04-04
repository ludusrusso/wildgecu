package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkill creates a skill directory with a SKILL.md file for testing.
func writeSkill(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, SkillFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseValid(t *testing.T) {
	data := []byte("---\nname: go-errors\ndescription: \"Go error handling\"\ntags:\n  - go\n  - errors\n---\n## Best Practices\nWrap errors.")
	s, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if s.Name != "go-errors" {
		t.Errorf("expected name go-errors, got %q", s.Name)
	}
	if s.Description != "Go error handling" {
		t.Errorf("expected description 'Go error handling', got %q", s.Description)
	}
	if len(s.Tags) != 2 || s.Tags[0] != "go" || s.Tags[1] != "errors" {
		t.Errorf("expected tags [go errors], got %v", s.Tags)
	}
	if s.Content != "## Best Practices\nWrap errors." {
		t.Errorf("unexpected content: %q", s.Content)
	}
}

func TestParseMissingFrontmatter(t *testing.T) {
	_, err := Parse([]byte("no frontmatter here"))
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestParseMissingClosingDelimiter(t *testing.T) {
	_, err := Parse([]byte("---\nname: test\ndescription: test\n"))
	if err == nil {
		t.Fatal("expected error for missing closing delimiter")
	}
}

func TestParseMissingName(t *testing.T) {
	_, err := Parse([]byte("---\ndescription: test\n---\ncontent"))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseMissingDescription(t *testing.T) {
	_, err := Parse([]byte("---\nname: test\n---\ncontent"))
	if err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	original := &Skill{
		Name:        "go-errors",
		Description: "Go error handling best practices",
		Tags:        []string{"go", "errors"},
		Content:     "## Best Practices\nWrap errors.",
	}

	data, err := Serialize(original)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse round-trip failed: %v", err)
	}

	if parsed.Name != original.Name {
		t.Errorf("name mismatch: %q vs %q", parsed.Name, original.Name)
	}
	if parsed.Description != original.Description {
		t.Errorf("description mismatch: %q vs %q", parsed.Description, original.Description)
	}
	if len(parsed.Tags) != len(original.Tags) {
		t.Errorf("tags length mismatch: %d vs %d", len(parsed.Tags), len(original.Tags))
	}
	if parsed.Content != original.Content {
		t.Errorf("content mismatch: %q vs %q", parsed.Content, original.Content)
	}
}

func TestSerializeNoTags(t *testing.T) {
	s := &Skill{
		Name:        "simple",
		Description: "A simple skill",
		Content:     "Some content",
	}

	data, err := Serialize(s)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	if strings.Contains(string(data), "tags:") {
		t.Error("expected no tags field in output")
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse round-trip failed: %v", err)
	}
	if parsed.Name != s.Name {
		t.Errorf("name mismatch: %q vs %q", parsed.Name, s.Name)
	}
}

func TestSkillPath(t *testing.T) {
	got := SkillPath("go-errors")
	if got != "go-errors/SKILL.md" {
		t.Errorf("expected go-errors/SKILL.md, got %q", got)
	}
}

func TestLoadAll(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "good", []byte("---\nname: good\ndescription: A good skill\n---\nGood content"))
	writeSkill(t, dir, "bad", []byte("---\nname: bad\n---\nmissing description"))
	writeSkill(t, dir, "also-good", []byte("---\nname: also-good\ndescription: Another good skill\ntags:\n  - test\n---\nMore content"))

	skills, errs := LoadAll(dir)

	if len(skills) != 2 {
		t.Fatalf("expected 2 valid skills, got %d", len(skills))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Error(), "bad") {
		t.Errorf("expected error about bad, got %q", errs[0])
	}
}

func TestLoadAllSkipsDirsWithoutSkillMD(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "valid", []byte("---\nname: valid\ndescription: Valid skill\n---\nContent"))
	os.MkdirAll(filepath.Join(dir, "noskill"), 0o755)
	os.WriteFile(filepath.Join(dir, "noskill", "other.txt"), []byte("not a skill"), 0o644)

	skills, errs := LoadAll(dir)

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d", len(errs))
	}
	if skills[0].Name != "valid" {
		t.Errorf("expected skill name 'valid', got %q", skills[0].Name)
	}
}

func TestLoadAllEmpty(t *testing.T) {
	dir := t.TempDir()
	skills, errs := LoadAll(dir)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "my-skill", []byte("---\nname: my-skill\ndescription: Test skill\n---\nContent here"))

	s, err := Load(dir, "my-skill")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if s.Name != "my-skill" {
		t.Errorf("expected name my-skill, got %q", s.Name)
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	s := &Skill{
		Name:        "new-skill",
		Description: "A new skill",
		Tags:        []string{"test"},
		Content:     "Content here",
	}

	if err := Save(dir, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was written correctly
	data, err := os.ReadFile(filepath.Join(dir, "new-skill", SkillFile))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsed.Name != "new-skill" {
		t.Errorf("expected name new-skill, got %q", parsed.Name)
	}
	if parsed.Description != "A new skill" {
		t.Errorf("expected description 'A new skill', got %q", parsed.Description)
	}
}

func TestSaveOverwrite(t *testing.T) {
	dir := t.TempDir()
	s := &Skill{
		Name:        "overwrite",
		Description: "Original",
		Content:     "Original content",
	}
	if err := Save(dir, s); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	s.Description = "Updated"
	s.Content = "Updated content"
	if err := Save(dir, s); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	loaded, err := Load(dir, "overwrite")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Description != "Updated" {
		t.Errorf("expected updated description, got %q", loaded.Description)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "to-delete", []byte("---\nname: to-delete\ndescription: Delete me\n---\nContent"))

	if err := Delete(dir, "to-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "to-delete")); !os.IsNotExist(err) {
		t.Error("expected directory to be deleted")
	}
}

func TestDeleteIdempotent(t *testing.T) {
	dir := t.TempDir()
	// Deleting a non-existent skill should not error
	if err := Delete(dir, "nonexistent"); err != nil {
		t.Fatalf("Delete of nonexistent skill should not error, got: %v", err)
	}
}
