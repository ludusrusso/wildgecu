package cron

import (
	"bytes"
	"fmt"
	"strings"

	"wildgecu/homer"

	"go.yaml.in/yaml/v3"
)

// CronJob represents a scheduled LLM prompt.
type CronJob struct {
	Name     string `yaml:"name"`
	Schedule string `yaml:"cron"`
	Prompt   string `yaml:"-"`
}

// Parse parses a cron job from a markdown file with YAML frontmatter.
// The format is:
//
//	---
//	name: daily-summary
//	cron: "0 9 * * *"
//	---
//	Your prompt here...
func Parse(data []byte) (*CronJob, error) {
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("cron: missing frontmatter delimiter")
	}

	// Find closing delimiter
	rest := content[4:] // skip opening "---\n"
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, fmt.Errorf("cron: missing closing frontmatter delimiter")
	}

	frontmatter := rest[:idx]
	body := strings.TrimSpace(rest[idx+4:]) // skip "\n---"

	var job CronJob
	if err := yaml.Unmarshal([]byte(frontmatter), &job); err != nil {
		return nil, fmt.Errorf("cron: parse frontmatter: %w", err)
	}

	if job.Name == "" {
		return nil, fmt.Errorf("cron: name is required")
	}
	if job.Schedule == "" {
		return nil, fmt.Errorf("cron: cron schedule is required")
	}

	job.Prompt = body
	return &job, nil
}

// Serialize writes a CronJob back to frontmatter+body format.
func Serialize(job *CronJob) ([]byte, error) {
	var buf bytes.Buffer

	fm, err := yaml.Marshal(struct {
		Name     string `yaml:"name"`
		Schedule string `yaml:"cron"`
	}{Name: job.Name, Schedule: job.Schedule})
	if err != nil {
		return nil, fmt.Errorf("cron: marshal frontmatter: %w", err)
	}

	buf.WriteString("---\n")
	buf.Write(fm)
	buf.WriteString("---\n")
	if job.Prompt != "" {
		buf.WriteString(job.Prompt)
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}

// Filename returns the markdown filename for a cron job name.
func Filename(name string) string {
	return name + ".md"
}

// LoadAll loads all cron jobs from a homer directory.
// It returns all successfully parsed jobs and any errors encountered.
func LoadAll(h homer.Homer) ([]*CronJob, []error) {
	files, err := h.Search("*.md")
	if err != nil {
		return nil, []error{fmt.Errorf("cron: search: %w", err)}
	}

	var jobs []*CronJob
	var errs []error

	for _, f := range files {
		data, err := h.Get(f)
		if err != nil {
			errs = append(errs, fmt.Errorf("cron: read %s: %w", f, err))
			continue
		}
		job, err := Parse(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("cron: %s: %w", f, err))
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, errs
}
