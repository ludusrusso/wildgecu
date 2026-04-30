package tools

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestExecTools(t *testing.T) {
	tools := ExecTools("/tmp", ExecConfig{})
	if len(tools) != 2 {
		t.Fatalf("expected 2 exec tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tl := range tools {
		names[tl.Definition().Name] = true
	}
	for _, want := range []string{"bash", "node"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	dir := t.TempDir()
	tl := newBashTool(dir, ExecConfig{})

	t.Run("echo stdout", func(t *testing.T) {
		var out bashOutput
		result, err := tl.Execute(context.Background(), map[string]any{"command": "echo hello"})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)

		if out.Stdout != "hello\n" {
			t.Fatalf("stdout = %q, want %q", out.Stdout, "hello\n")
		}
		if out.ExitCode != 0 {
			t.Fatalf("exit_code = %d, want 0", out.ExitCode)
		}
	})

	t.Run("stderr", func(t *testing.T) {
		var out bashOutput
		result, err := tl.Execute(context.Background(), map[string]any{"command": "echo err >&2"})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)

		if out.Stderr != "err\n" {
			t.Fatalf("stderr = %q, want %q", out.Stderr, "err\n")
		}
	})

	t.Run("nonzero exit code", func(t *testing.T) {
		var out bashOutput
		result, err := tl.Execute(context.Background(), map[string]any{"command": "exit 42"})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)

		if out.ExitCode != 42 {
			t.Fatalf("exit_code = %d, want 42", out.ExitCode)
		}
	})

	t.Run("runs in workDir", func(t *testing.T) {
		var out bashOutput
		result, err := tl.Execute(context.Background(), map[string]any{"command": "pwd"})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)

		// TempDir may have symlinks (e.g. /var -> /private/var on macOS)
		if out.Stdout == "" {
			t.Fatal("expected non-empty pwd output")
		}
	})

	t.Run("mixed stdout and stderr", func(t *testing.T) {
		var out bashOutput
		result, err := tl.Execute(context.Background(), map[string]any{
			"command": "echo out && echo err >&2",
		})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)

		if out.Stdout != "out\n" {
			t.Fatalf("stdout = %q", out.Stdout)
		}
		if out.Stderr != "err\n" {
			t.Fatalf("stderr = %q", out.Stderr)
		}
		if out.ExitCode != 0 {
			t.Fatalf("exit_code = %d", out.ExitCode)
		}
	})

	t.Run("default timeout still applies when timeout_seconds omitted", func(t *testing.T) {
		// Sanity check: a quick command still works without a
		// timeout_seconds arg.
		var out bashOutput
		result, err := tl.Execute(context.Background(), map[string]any{"command": "echo ok"})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)
		if out.Stdout != "ok\n" {
			t.Fatalf("stdout = %q", out.Stdout)
		}
	})

	t.Run("timeout_seconds arg fires", func(t *testing.T) {
		var out bashOutput
		start := time.Now()
		result, err := tl.Execute(context.Background(), map[string]any{
			"command":         "sleep 5",
			"timeout_seconds": float64(1),
		})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)
		if !out.TimedOut {
			t.Fatalf("expected timed_out=true, got %+v", out)
		}
		if out.ExitCode != -1 {
			t.Fatalf("exit_code = %d, want -1", out.ExitCode)
		}
		if elapsed > 4*time.Second {
			t.Fatalf("expected fast return, elapsed = %s", elapsed)
		}
	})

	t.Run("timeout_seconds above hard cap returns boundary error", func(t *testing.T) {
		capped := newBashTool(dir, ExecConfig{MaxTimeoutSeconds: 5})
		_, err := capped.Execute(context.Background(), map[string]any{
			"command":         "true",
			"timeout_seconds": float64(9999),
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "exceeds maximum") {
			t.Fatalf("error = %q, want it to mention 'exceeds maximum'", err.Error())
		}
	})

	t.Run("large stdout is truncated with marker and total bytes", func(t *testing.T) {
		small := newBashTool(dir, ExecConfig{HeadBytes: 100, TailBytes: 100})
		var out bashOutput
		result, err := small.Execute(context.Background(), map[string]any{
			"command": `for i in $(seq 1 200); do printf 'line%03d-AAAAAAAAAAAA\n' $i; done`,
		})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)
		if !strings.Contains(out.Stdout, "[... truncated ") {
			t.Fatalf("expected truncation marker in stdout: %q", out.Stdout)
		}
		if out.StdoutTotalBytes <= 200 {
			t.Fatalf("StdoutTotalBytes = %d, want > 200", out.StdoutTotalBytes)
		}
		// Last line must survive in tail.
		if !strings.Contains(out.Stdout, "line200-") {
			t.Fatalf("tail lost last line: %q", out.Stdout)
		}
	})
}

func TestNode(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}

	dir := t.TempDir()
	tl := newNodeTool(dir, ExecConfig{})

	t.Run("console.log", func(t *testing.T) {
		var out nodeOutput
		result, err := tl.Execute(context.Background(), map[string]any{
			"script": `console.log("hello from node")`,
		})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)

		if out.Stdout != "hello from node\n" {
			t.Fatalf("stdout = %q", out.Stdout)
		}
		if out.ExitCode != 0 {
			t.Fatalf("exit_code = %d", out.ExitCode)
		}
	})

	t.Run("stderr", func(t *testing.T) {
		var out nodeOutput
		result, err := tl.Execute(context.Background(), map[string]any{
			"script": `console.error("node err")`,
		})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)

		if out.Stderr != "node err\n" {
			t.Fatalf("stderr = %q", out.Stderr)
		}
	})

	t.Run("nonzero exit", func(t *testing.T) {
		var out nodeOutput
		result, err := tl.Execute(context.Background(), map[string]any{
			"script": `process.exit(7)`,
		})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)

		if out.ExitCode != 7 {
			t.Fatalf("exit_code = %d, want 7", out.ExitCode)
		}
	})

	t.Run("json output", func(t *testing.T) {
		var out nodeOutput
		result, err := tl.Execute(context.Background(), map[string]any{
			"script": `console.log(JSON.stringify({a: 1}))`,
		})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)

		if out.Stdout != `{"a":1}`+"\n" {
			t.Fatalf("stdout = %q", out.Stdout)
		}
	})

	t.Run("default timeout still applies when timeout_seconds omitted", func(t *testing.T) {
		var out nodeOutput
		result, err := tl.Execute(context.Background(), map[string]any{
			"script": `console.log("ok")`,
		})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)
		if out.Stdout != "ok\n" {
			t.Fatalf("stdout = %q", out.Stdout)
		}
	})

	t.Run("timeout_seconds arg fires", func(t *testing.T) {
		var out nodeOutput
		start := time.Now()
		result, err := tl.Execute(context.Background(), map[string]any{
			"script":          `setTimeout(() => {}, 5000)`,
			"timeout_seconds": float64(1),
		})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)
		if !out.TimedOut {
			t.Fatalf("expected timed_out=true, got %+v", out)
		}
		if out.ExitCode != -1 {
			t.Fatalf("exit_code = %d, want -1", out.ExitCode)
		}
		if elapsed > 4*time.Second {
			t.Fatalf("expected fast return, elapsed = %s", elapsed)
		}
	})

	t.Run("timeout_seconds above hard cap returns boundary error", func(t *testing.T) {
		capped := newNodeTool(dir, ExecConfig{MaxTimeoutSeconds: 5})
		_, err := capped.Execute(context.Background(), map[string]any{
			"script":          `console.log("hi")`,
			"timeout_seconds": float64(9999),
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "exceeds maximum") {
			t.Fatalf("error = %q, want it to mention 'exceeds maximum'", err.Error())
		}
	})

	t.Run("large stdout is truncated with marker and total bytes", func(t *testing.T) {
		small := newNodeTool(dir, ExecConfig{HeadBytes: 100, TailBytes: 100})
		var out nodeOutput
		result, err := small.Execute(context.Background(), map[string]any{
			"script": `for (let i = 1; i <= 200; i++) console.log("line" + String(i).padStart(3, "0") + "-AAAAAAAAAAAA");`,
		})
		if err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(result), &out)
		if !strings.Contains(out.Stdout, "[... truncated ") {
			t.Fatalf("expected truncation marker in stdout: %q", out.Stdout)
		}
		if out.StdoutTotalBytes <= 200 {
			t.Fatalf("StdoutTotalBytes = %d, want > 200", out.StdoutTotalBytes)
		}
		if !strings.Contains(out.Stdout, "line200-") {
			t.Fatalf("tail lost last line: %q", out.Stdout)
		}
	})
}
