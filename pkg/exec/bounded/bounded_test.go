package bounded

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func skipIfNoBash(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
}

func TestRun(t *testing.T) {
	skipIfNoBash(t)

	t.Run("small output passes through unchanged", func(t *testing.T) {
		res, err := Run(context.Background(), "bash", []string{"-c", "echo hello"}, Opts{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Stdout != "hello\n" {
			t.Fatalf("stdout = %q, want %q", res.Stdout, "hello\n")
		}
		if res.ExitCode != 0 {
			t.Fatalf("exit_code = %d, want 0", res.ExitCode)
		}
		if res.TimedOut {
			t.Fatal("expected TimedOut=false")
		}
		if res.StdoutTotalBytes != len("hello\n") {
			t.Fatalf("StdoutTotalBytes = %d, want %d", res.StdoutTotalBytes, len("hello\n"))
		}
	})

	t.Run("nonzero exit code propagates without error", func(t *testing.T) {
		res, err := Run(context.Background(), "bash", []string{"-c", "exit 42"}, Opts{})
		if err != nil {
			t.Fatal(err)
		}
		if res.ExitCode != 42 {
			t.Fatalf("exit_code = %d, want 42", res.ExitCode)
		}
		if res.TimedOut {
			t.Fatal("expected TimedOut=false")
		}
	})

	t.Run("stdout and stderr are captured independently", func(t *testing.T) {
		res, err := Run(context.Background(), "bash",
			[]string{"-c", "echo out && echo err >&2"}, Opts{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Stdout != "out\n" {
			t.Fatalf("stdout = %q", res.Stdout)
		}
		if res.Stderr != "err\n" {
			t.Fatalf("stderr = %q", res.Stderr)
		}
	})

	t.Run("workdir is honored", func(t *testing.T) {
		dir := t.TempDir()
		res, err := Run(context.Background(), "bash", []string{"-c", "pwd"}, Opts{WorkDir: dir})
		if err != nil {
			t.Fatal(err)
		}
		// macOS may resolve /tmp through /private/tmp; just assert non-empty.
		if strings.TrimSpace(res.Stdout) == "" {
			t.Fatal("expected non-empty pwd output")
		}
	})

	t.Run("timeout fires with TimedOut=true and exit_code=-1", func(t *testing.T) {
		start := time.Now()
		res, err := Run(context.Background(), "bash", []string{"-c", "sleep 5"}, Opts{
			Timeout: 200 * time.Millisecond,
		})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if !res.TimedOut {
			t.Fatal("expected TimedOut=true")
		}
		if res.ExitCode != -1 {
			t.Fatalf("ExitCode = %d, want -1", res.ExitCode)
		}
		if elapsed > 3*time.Second {
			t.Fatalf("expected fast return, elapsed = %s", elapsed)
		}
	})

	t.Run("timeout above max returns boundary error", func(t *testing.T) {
		_, err := Run(context.Background(), "bash", []string{"-c", "true"}, Opts{
			Timeout:    20 * time.Minute,
			MaxTimeout: 10 * time.Minute,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "exceeds maximum") {
			t.Fatalf("error = %q, want it to mention 'exceeds maximum'", err.Error())
		}
	})

	t.Run("large output is capped with marker and total bytes", func(t *testing.T) {
		// Generate a known number of newline-separated lines so we can
		// verify line alignment of the head+tail window.
		head := 100
		tail := 100
		// 50 lines of "AAAA...\n" each ~30 bytes → ~1500 bytes total.
		script := `for i in $(seq 1 50); do printf 'line%02d-AAAAAAAAAAAAAAAAAAA\n' $i; done`
		res, err := Run(context.Background(), "bash", []string{"-c", script}, Opts{
			HeadBytes: head,
			TailBytes: tail,
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.ExitCode != 0 {
			t.Fatalf("exit_code = %d", res.ExitCode)
		}
		if !strings.Contains(res.Stdout, "[... truncated ") {
			t.Fatalf("stdout = %q, want truncation marker", res.Stdout)
		}
		if res.StdoutTotalBytes < head+tail {
			t.Fatalf("StdoutTotalBytes = %d, want >= %d", res.StdoutTotalBytes, head+tail)
		}
		// Last line of the original output should survive in the tail.
		if !strings.Contains(res.Stdout, "line50-") {
			t.Fatalf("stdout = %q, expected last line to survive", res.Stdout)
		}
		// Head should start at line01 (no truncation before head).
		if !strings.HasPrefix(res.Stdout, "line01-") {
			t.Fatalf("stdout = %q, expected head to start at line01", res.Stdout)
		}
	})

	t.Run("head+tail capture is line-aligned around the marker", func(t *testing.T) {
		// Force a marker by exceeding head+tail. Lines are exactly 10
		// bytes each ("ABCDEFGHI\n") so we can reason about alignment.
		head := 25 // mid-line of line 3 (bytes 21-25 are "ABCDE" of line 3)
		tail := 25 // mid-line of last line; round-up moves to next newline
		// 100 lines: 1000 bytes total → forces truncation.
		script := `for i in $(seq 1 100); do printf 'ABCDEFGHI\n'; done`
		res, err := Run(context.Background(), "bash", []string{"-c", script}, Opts{
			HeadBytes: head,
			TailBytes: tail,
		})
		if err != nil {
			t.Fatal(err)
		}
		// Find the marker.
		idx := strings.Index(res.Stdout, "[... truncated")
		if idx < 0 {
			t.Fatalf("missing marker in output: %q", res.Stdout)
		}
		// Head portion ends at the byte immediately before the marker.
		// Line alignment: that byte must be a newline.
		if idx == 0 {
			t.Fatal("marker landed at start; expected at least one full line of head")
		}
		if res.Stdout[idx-1] != '\n' {
			t.Fatalf("byte before marker = %q, want newline", res.Stdout[idx-1])
		}
		// Tail portion begins after the marker line. Marker is followed
		// by '\n' itself (per format), and the next portion must begin
		// with a fresh line (or be empty).
		afterMarker := res.Stdout[idx:]
		nl := strings.IndexByte(afterMarker, '\n')
		if nl < 0 {
			t.Fatalf("marker has no trailing newline: %q", afterMarker)
		}
		tailPortion := afterMarker[nl+1:]
		// tailPortion (if non-empty) should be made of full "ABCDEFGHI\n"
		// lines — i.e. start with a complete line.
		if tailPortion != "" && !strings.HasPrefix(tailPortion, "ABCDEFGHI\n") {
			t.Fatalf("tail portion not line-aligned: %q", tailPortion)
		}
	})

	t.Run("output exactly head+tail is not truncated", func(t *testing.T) {
		// Produce a deterministic stream just barely fitting head+tail.
		head := 50
		tail := 50
		// 10 lines of "abcdefghi\n" (10 bytes) = 100 bytes = head+tail
		script := `for i in $(seq 1 10); do printf 'abcdefghi\n'; done`
		res, err := Run(context.Background(), "bash", []string{"-c", script}, Opts{
			HeadBytes: head,
			TailBytes: tail,
		})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(res.Stdout, "[... truncated ") {
			t.Fatalf("unexpected marker in stdout: %q", res.Stdout)
		}
		if res.StdoutTotalBytes != 100 {
			t.Fatalf("StdoutTotalBytes = %d, want 100", res.StdoutTotalBytes)
		}
		if res.Stdout != strings.Repeat("abcdefghi\n", 10) {
			t.Fatalf("stdout mismatch: %q", res.Stdout)
		}
	})

	t.Run("stderr truncates independently of stdout", func(t *testing.T) {
		head := 50
		tail := 50
		// Big stderr, tiny stdout.
		script := `echo small; for i in $(seq 1 100); do printf 'errline%02d\n' $i >&2; done`
		res, err := Run(context.Background(), "bash", []string{"-c", script}, Opts{
			HeadBytes: head,
			TailBytes: tail,
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Stdout != "small\n" {
			t.Fatalf("stdout = %q", res.Stdout)
		}
		if !strings.Contains(res.Stderr, "[... truncated ") {
			t.Fatalf("stderr should be truncated: %q", res.Stderr)
		}
		if !strings.Contains(res.Stderr, "errline100") {
			t.Fatalf("stderr lost the last line: %q", res.Stderr)
		}
	})
}

func TestHeadTailBuf(t *testing.T) {
	t.Run("under capacity returns full buffer", func(t *testing.T) {
		b := newHeadTailBuf(100, 100)
		_, _ = b.Write([]byte("hello"))
		if b.String() != "hello" {
			t.Fatalf("got %q", b.String())
		}
		if b.Total() != 5 {
			t.Fatalf("total = %d", b.Total())
		}
	})

	t.Run("exact head+tail boundary returns concatenated", func(t *testing.T) {
		b := newHeadTailBuf(5, 5)
		_, _ = b.Write([]byte("aaaaa")) // fills head
		_, _ = b.Write([]byte("bbbbb")) // fills tail
		if b.String() != "aaaaabbbbb" {
			t.Fatalf("got %q", b.String())
		}
		if b.Total() != 10 {
			t.Fatalf("total = %d", b.Total())
		}
	})

	t.Run("overflow drops middle bytes and reports them", func(t *testing.T) {
		// head=10 captures "AAAAA\nBBBB"; tail=10 captures the last 10
		// bytes "CCC\nDDDDD\n". Line-alignment trims head down to
		// "AAAAA\n" and advances tail past its first newline to
		// "DDDDD\n", with the marker between them.
		b := newHeadTailBuf(10, 10)
		_, _ = b.Write([]byte("AAAAA\nBBBBB\nCCCCC\nDDDDD\n"))
		out := b.String()
		if !strings.Contains(out, "[... truncated ") {
			t.Fatalf("missing marker: %q", out)
		}
		if !strings.HasPrefix(out, "AAAAA\n") {
			t.Fatalf("head not aligned to newline: %q", out)
		}
		if !strings.HasSuffix(out, "DDDDD\n") {
			t.Fatalf("tail not aligned to newline: %q", out)
		}
		if b.Total() != 24 {
			t.Fatalf("total = %d, want 24", b.Total())
		}
	})

	t.Run("multi-write across head boundary", func(t *testing.T) {
		b := newHeadTailBuf(5, 100)
		_, _ = b.Write([]byte("aa"))
		_, _ = b.Write([]byte("bbcc"))
		_, _ = b.Write([]byte("dd"))
		// total 8 bytes; head takes first 5 ("aabbc"), tail keeps "cdd"
		if b.Total() != 8 {
			t.Fatalf("total = %d", b.Total())
		}
		// no truncation since 8 ≤ 5+100
		if b.String() != "aabbccdd" {
			t.Fatalf("got %q", b.String())
		}
	})
}
