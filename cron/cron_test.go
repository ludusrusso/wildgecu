package cron

import (
	"strings"
	"testing"

	"wildgecu/homer"
)

func TestParseValid(t *testing.T) {
	data := []byte("---\nname: daily-summary\ncron: \"0 9 * * *\"\n---\nSummarize my day")
	job, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if job.Name != "daily-summary" {
		t.Errorf("expected name daily-summary, got %q", job.Name)
	}
	if job.Schedule != "0 9 * * *" {
		t.Errorf("expected schedule '0 9 * * *', got %q", job.Schedule)
	}
	if job.Prompt != "Summarize my day" {
		t.Errorf("expected prompt 'Summarize my day', got %q", job.Prompt)
	}
}

func TestParseMissingFrontmatter(t *testing.T) {
	_, err := Parse([]byte("no frontmatter here"))
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestParseMissingClosingDelimiter(t *testing.T) {
	_, err := Parse([]byte("---\nname: test\ncron: \"* * * * *\"\n"))
	if err == nil {
		t.Fatal("expected error for missing closing delimiter")
	}
}

func TestParseMissingName(t *testing.T) {
	_, err := Parse([]byte("---\ncron: \"0 9 * * *\"\n---\nprompt"))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseMissingSchedule(t *testing.T) {
	_, err := Parse([]byte("---\nname: test\n---\nprompt"))
	if err == nil {
		t.Fatal("expected error for missing schedule")
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	original := &CronJob{
		Name:     "weekly-report",
		Schedule: "0 0 * * 1",
		Prompt:   "Generate weekly report",
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
	if parsed.Schedule != original.Schedule {
		t.Errorf("schedule mismatch: %q vs %q", parsed.Schedule, original.Schedule)
	}
	if parsed.Prompt != original.Prompt {
		t.Errorf("prompt mismatch: %q vs %q", parsed.Prompt, original.Prompt)
	}
}

func TestFilename(t *testing.T) {
	if got := Filename("daily-summary"); got != "daily-summary.md" {
		t.Errorf("expected daily-summary.md, got %q", got)
	}
}

func TestLoadAll(t *testing.T) {
	h := homer.NewMem()

	// Valid job
	h.Upsert("good.md", []byte("---\nname: good\ncron: \"0 9 * * *\"\n---\nDo something"))

	// Invalid job (missing schedule)
	h.Upsert("bad.md", []byte("---\nname: bad\n---\nmissing schedule"))

	// Another valid job
	h.Upsert("also-good.md", []byte("---\nname: also-good\ncron: \"*/5 * * * *\"\n---\nDo another thing"))

	jobs, errs := LoadAll(h)

	if len(jobs) != 2 {
		t.Fatalf("expected 2 valid jobs, got %d", len(jobs))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Error(), "bad.md") {
		t.Errorf("expected error about bad.md, got %q", errs[0])
	}
}

func TestLoadAllEmpty(t *testing.T) {
	h := homer.NewMem()
	jobs, errs := LoadAll(h)
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}
