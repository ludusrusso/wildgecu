package cron

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// CronJob represents a scheduled LLM prompt.
type CronJob struct {
	Name      string `yaml:"name"`
	Schedule  string `yaml:"cron"`
	Suspended bool   `yaml:"suspended,omitempty"`
	Prompt    string `yaml:"-"`
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
		Name      string `yaml:"name"`
		Schedule  string `yaml:"cron"`
		Suspended bool   `yaml:"suspended,omitempty"`
	}{Name: job.Name, Schedule: job.Schedule, Suspended: job.Suspended})
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

// LoadAll loads all cron jobs from a directory path.
// It returns all successfully parsed jobs and any errors encountered.
func LoadAll(dir string) ([]*CronJob, []error) {
	results := LoadAllResults(dir)
	var jobs []*CronJob
	var errs []error
	for _, r := range results {
		if r.Err != nil {
			errs = append(errs, r.Err)
			continue
		}
		jobs = append(jobs, r.Job)
	}
	return jobs, errs
}

// LoadResult is a per-file outcome from loading a cron directory. Successfully
// parsed files populate Job; unparseable files populate Err. Name is derived
// from the frontmatter when parseable, else from the filename stem.
type LoadResult struct {
	Name string
	Path string
	Job  *CronJob
	Err  error
}

// LoadAllResults walks a cron directory and returns one LoadResult per .md
// file, including unparseable ones. Unlike LoadAll it retains the filename
// so callers can surface broken jobs by name.
func LoadAllResults(dir string) []LoadResult {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return []LoadResult{{Err: fmt.Errorf("cron: search: %w", err)}}
	}

	results := make([]LoadResult, 0, len(matches))
	for _, path := range matches {
		f := filepath.Base(path)
		stem := strings.TrimSuffix(f, filepath.Ext(f))
		data, err := os.ReadFile(path)
		if err != nil {
			results = append(results, LoadResult{Name: stem, Path: path, Err: fmt.Errorf("cron: read %s: %w", f, err)})
			continue
		}
		job, err := Parse(data)
		if err != nil {
			results = append(results, LoadResult{Name: stem, Path: path, Err: fmt.Errorf("cron: %s: %w", f, err)})
			continue
		}
		results = append(results, LoadResult{Name: job.Name, Path: path, Job: job})
	}

	return results
}
