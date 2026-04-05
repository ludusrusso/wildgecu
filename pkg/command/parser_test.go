package command

import "testing"

func TestParse(t *testing.T) {
	t.Run("command without args", func(t *testing.T) {
		name, args := Parse("/help")
		if name != "help" {
			t.Errorf("expected name %q, got %q", "help", name)
		}
		if args != "" {
			t.Errorf("expected empty args, got %q", args)
		}
	})

	t.Run("command with args", func(t *testing.T) {
		name, args := Parse("/skill install my-skill")
		if name != "skill" {
			t.Errorf("expected name %q, got %q", "skill", name)
		}
		if args != "install my-skill" {
			t.Errorf("expected args %q, got %q", "install my-skill", args)
		}
	})

	t.Run("command with leading spaces in args", func(t *testing.T) {
		name, args := Parse("/echo   hello")
		if name != "echo" {
			t.Errorf("expected name %q, got %q", "echo", name)
		}
		if args != "  hello" {
			t.Errorf("expected args %q, got %q", "  hello", args)
		}
	})

	t.Run("slash only", func(t *testing.T) {
		name, args := Parse("/")
		if name != "" {
			t.Errorf("expected empty name, got %q", name)
		}
		if args != "" {
			t.Errorf("expected empty args, got %q", args)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		name, args := Parse("")
		if name != "" {
			t.Errorf("expected empty name, got %q", name)
		}
		if args != "" {
			t.Errorf("expected empty args, got %q", args)
		}
	})

	t.Run("no slash prefix", func(t *testing.T) {
		name, args := Parse("hello world")
		if name != "" {
			t.Errorf("expected empty name, got %q", name)
		}
		if args != "" {
			t.Errorf("expected empty args, got %q", args)
		}
	})
}
