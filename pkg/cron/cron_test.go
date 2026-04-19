package cron

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestParseSuspended(t *testing.T) {
	t.Run("explicit true", func(t *testing.T) {
		data := []byte("---\nname: foo\ncron: \"0 9 * * *\"\nsuspended: true\n---\nprompt")
		job, err := Parse(data)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if !job.Suspended {
			t.Errorf("expected Suspended=true, got false")
		}
	})

	t.Run("explicit false", func(t *testing.T) {
		data := []byte("---\nname: foo\ncron: \"0 9 * * *\"\nsuspended: false\n---\nprompt")
		job, err := Parse(data)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if job.Suspended {
			t.Errorf("expected Suspended=false, got true")
		}
	})

	t.Run("absent defaults to false", func(t *testing.T) {
		data := []byte("---\nname: foo\ncron: \"0 9 * * *\"\n---\nprompt")
		job, err := Parse(data)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if job.Suspended {
			t.Errorf("expected Suspended=false when absent, got true")
		}
	})
}

func TestSerializeSuspended(t *testing.T) {
	t.Run("true round-trip", func(t *testing.T) {
		original := &CronJob{
			Name:      "foo",
			Schedule:  "0 9 * * *",
			Suspended: true,
			Prompt:    "hello",
		}
		data, err := Serialize(original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}
		if !strings.Contains(string(data), "suspended: true") {
			t.Errorf("expected serialized output to contain 'suspended: true', got:\n%s", data)
		}
		parsed, err := Parse(data)
		if err != nil {
			t.Fatalf("Parse round-trip failed: %v", err)
		}
		if !parsed.Suspended {
			t.Errorf("expected Suspended=true after round-trip")
		}
	})

	t.Run("false omits the field", func(t *testing.T) {
		original := &CronJob{
			Name:     "foo",
			Schedule: "0 9 * * *",
			Prompt:   "hello",
		}
		data, err := Serialize(original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}
		if strings.Contains(string(data), "suspended") {
			t.Errorf("expected serialized output to omit 'suspended' when false, got:\n%s", data)
		}
	})
}

func TestParseTimeout(t *testing.T) {
	t.Run("parses Go duration strings", func(t *testing.T) {
		cases := map[string]time.Duration{
			"30m":    30 * time.Minute,
			"2h":     2 * time.Hour,
			"1h30m":  90 * time.Minute,
			"500ms":  500 * time.Millisecond,
		}
		for in, want := range cases {
			t.Run(in, func(t *testing.T) {
				data := []byte("---\nname: foo\ncron: \"0 9 * * *\"\ntimeout: " + in + "\n---\nprompt")
				job, err := Parse(data)
				if err != nil {
					t.Fatalf("Parse failed: %v", err)
				}
				if job.Timeout != want {
					t.Errorf("expected Timeout=%s, got %s", want, job.Timeout)
				}
			})
		}
	})

	t.Run("absent defaults to zero", func(t *testing.T) {
		data := []byte("---\nname: foo\ncron: \"0 9 * * *\"\n---\nprompt")
		job, err := Parse(data)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if job.Timeout != 0 {
			t.Errorf("expected Timeout=0 when absent, got %s", job.Timeout)
		}
	})

	t.Run("invalid duration errors", func(t *testing.T) {
		data := []byte("---\nname: foo\ncron: \"0 9 * * *\"\ntimeout: not-a-duration\n---\nprompt")
		if _, err := Parse(data); err == nil {
			t.Fatal("expected error for invalid timeout")
		}
	})

	t.Run("negative duration errors", func(t *testing.T) {
		data := []byte("---\nname: foo\ncron: \"0 9 * * *\"\ntimeout: -5m\n---\nprompt")
		if _, err := Parse(data); err == nil {
			t.Fatal("expected error for negative timeout")
		}
	})
}

func TestSerializeTimeout(t *testing.T) {
	t.Run("non-zero round-trips", func(t *testing.T) {
		original := &CronJob{
			Name:     "foo",
			Schedule: "0 9 * * *",
			Timeout:  30 * time.Minute,
			Prompt:   "hello",
		}
		data, err := Serialize(original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}
		if !strings.Contains(string(data), "timeout: 30m0s") {
			t.Errorf("expected serialized output to contain 'timeout: 30m0s', got:\n%s", data)
		}
		parsed, err := Parse(data)
		if err != nil {
			t.Fatalf("Parse round-trip failed: %v", err)
		}
		if parsed.Timeout != 30*time.Minute {
			t.Errorf("expected Timeout=30m after round-trip, got %s", parsed.Timeout)
		}
	})

	t.Run("zero omits the field", func(t *testing.T) {
		original := &CronJob{
			Name:     "foo",
			Schedule: "0 9 * * *",
			Prompt:   "hello",
		}
		data, err := Serialize(original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}
		if strings.Contains(string(data), "timeout") {
			t.Errorf("expected serialized output to omit 'timeout' when zero, got:\n%s", data)
		}
	})
}

func TestFilename(t *testing.T) {
	if got := Filename("daily-summary"); got != "daily-summary.md" {
		t.Errorf("expected daily-summary.md, got %q", got)
	}
}

func TestLoadAll(t *testing.T) {
	dir := t.TempDir()

	// Valid job
	os.WriteFile(filepath.Join(dir, "good.md"), []byte("---\nname: good\ncron: \"0 9 * * *\"\n---\nDo something"), 0o644)

	// Invalid job (missing schedule)
	os.WriteFile(filepath.Join(dir, "bad.md"), []byte("---\nname: bad\n---\nmissing schedule"), 0o644)

	// Another valid job
	os.WriteFile(filepath.Join(dir, "also-good.md"), []byte("---\nname: also-good\ncron: \"*/5 * * * *\"\n---\nDo another thing"), 0o644)

	jobs, errs := LoadAll(dir)

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
	dir := t.TempDir()
	jobs, errs := LoadAll(dir)
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}
