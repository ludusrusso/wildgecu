package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// SkillFile is the standard filename for a skill definition.
const SkillFile = "SKILL.md"

// Skill represents a domain-specific knowledge file.
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	Content     string   `yaml:"-"`
}

// Parse parses a skill from a markdown file with YAML frontmatter.
// The format is:
//
//	---
//	name: go-error-handling
//	description: "Best practices for Go error handling"
//	tags:
//	  - go
//	  - errors
//	---
//	## Go Error Handling Best Practices
//	...skill content as markdown...
func Parse(data []byte) (*Skill, error) {
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("skill: missing frontmatter delimiter")
	}

	// Find closing delimiter
	rest := content[4:] // skip opening "---\n"
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, fmt.Errorf("skill: missing closing frontmatter delimiter")
	}

	frontmatter := rest[:idx]
	body := strings.TrimSpace(rest[idx+4:]) // skip "\n---"

	var s Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &s); err != nil {
		return nil, fmt.Errorf("skill: parse frontmatter: %w", err)
	}

	if s.Name == "" {
		return nil, fmt.Errorf("skill: name is required")
	}
	if s.Description == "" {
		return nil, fmt.Errorf("skill: description is required")
	}

	s.Content = body
	return &s, nil
}

// Serialize writes a Skill back to frontmatter+body format.
func Serialize(s *Skill) ([]byte, error) {
	var buf bytes.Buffer

	fm, err := yaml.Marshal(struct {
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		Tags        []string `yaml:"tags,omitempty"`
	}{Name: s.Name, Description: s.Description, Tags: s.Tags})
	if err != nil {
		return nil, fmt.Errorf("skill: marshal frontmatter: %w", err)
	}

	buf.WriteString("---\n")
	buf.Write(fm)
	buf.WriteString("---\n")
	if s.Content != "" {
		buf.WriteString(s.Content)
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}

// SkillPath returns the path to a skill's SKILL.md file within the skills home.
func SkillPath(name string) string {
	return filepath.Join(name, SkillFile)
}

// LoadAll loads all skills from a directory.
// Each skill is expected to be a subdirectory containing a SKILL.md file.
// It returns all successfully parsed skills and any errors encountered.
func LoadAll(dir string) ([]*Skill, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("skill: list dirs: %w", err)}
	}

	var skills []*Skill
	var errs []error

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name(), SkillFile))
		if err != nil {
			// Skip directories without SKILL.md (not an error)
			continue
		}
		s, err := Parse(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("skill: %s: %w", e.Name(), err))
			continue
		}
		skills = append(skills, s)
	}

	return skills, errs
}

// Load loads a single skill by name from a directory.
func Load(dir string, name string) (*Skill, error) {
	data, err := os.ReadFile(filepath.Join(dir, name, SkillFile))
	if err != nil {
		return nil, fmt.Errorf("skill: load %s: %w", name, err)
	}
	return Parse(data)
}

// Save writes a skill to dir/<name>/SKILL.md, creating directories as needed.
func Save(dir string, s *Skill) error {
	data, err := Serialize(s)
	if err != nil {
		return err
	}
	skillDir := filepath.Join(dir, s.Name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return fmt.Errorf("skill: create dir: %w", err)
	}
	return os.WriteFile(filepath.Join(skillDir, SkillFile), data, 0o644)
}

// Delete removes a skill directory. It is idempotent — no error if the skill
// does not exist.
func Delete(dir string, name string) error {
	return os.RemoveAll(filepath.Join(dir, name))
}
