package skill

import (
	"bytes"
	"fmt"
	"strings"

	"wildgecu/homer"

	"go.yaml.in/yaml/v3"
)

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

// Filename returns the markdown filename for a skill name.
func Filename(name string) string {
	return name + ".md"
}

// LoadAll loads all skills from a homer directory.
// It returns all successfully parsed skills and any errors encountered.
func LoadAll(h homer.Homer) ([]*Skill, []error) {
	files, err := h.Search("*.md")
	if err != nil {
		return nil, []error{fmt.Errorf("skill: search: %w", err)}
	}

	var skills []*Skill
	var errs []error

	for _, f := range files {
		data, err := h.Get(f)
		if err != nil {
			errs = append(errs, fmt.Errorf("skill: read %s: %w", f, err))
			continue
		}
		s, err := Parse(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("skill: %s: %w", f, err))
			continue
		}
		skills = append(skills, s)
	}

	return skills, errs
}

// Load loads a single skill by name from a homer directory.
func Load(h homer.Homer, name string) (*Skill, error) {
	data, err := h.Get(Filename(name))
	if err != nil {
		return nil, fmt.Errorf("skill: load %s: %w", name, err)
	}
	return Parse(data)
}
