package telegram

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"wildgecu/pkg/command"
	"wildgecu/pkg/skill"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestBuildBotCommands(t *testing.T) {
	t.Run("basic conversion", func(t *testing.T) {
		entries := []command.Entry{
			{Name: "help", Description: "Show help"},
			{Name: "status", Description: "Show status"},
		}
		cmds := buildBotCommands(entries)
		if len(cmds) != 2 {
			t.Fatalf("expected 2 commands, got %d", len(cmds))
		}
		if cmds[0].Command != "help" || cmds[0].Description != "Show help" {
			t.Errorf("unexpected first command: %+v", cmds[0])
		}
		if cmds[1].Command != "status" || cmds[1].Description != "Show status" {
			t.Errorf("unexpected second command: %+v", cmds[1])
		}
	})

	t.Run("empty list", func(t *testing.T) {
		cmds := buildBotCommands(nil)
		if len(cmds) != 0 {
			t.Errorf("expected 0 commands, got %d", len(cmds))
		}
	})

	t.Run("truncates description to 256 chars", func(t *testing.T) {
		long := strings.Repeat("a", 300)
		entries := []command.Entry{{Name: "test", Description: long}}
		cmds := buildBotCommands(entries)
		if len(cmds) != 1 {
			t.Fatalf("expected 1 command, got %d", len(cmds))
		}
		if len(cmds[0].Description) > 256 {
			t.Errorf("description length %d exceeds 256", len(cmds[0].Description))
		}
		// Should end with ellipsis to indicate truncation.
		if !strings.HasSuffix(cmds[0].Description, "...") {
			t.Errorf("truncated description should end with '...', got %q", cmds[0].Description[250:])
		}
	})

	t.Run("limits to 100 commands", func(t *testing.T) {
		entries := make([]command.Entry, 120)
		for i := range entries {
			entries[i] = command.Entry{Name: "cmd" + strings.Repeat("x", 2), Description: "desc"}
		}
		cmds := buildBotCommands(entries)
		if len(cmds) > 100 {
			t.Errorf("expected at most 100 commands, got %d", len(cmds))
		}
	})

	t.Run("empty description gets default", func(t *testing.T) {
		entries := []command.Entry{{Name: "test", Description: ""}}
		cmds := buildBotCommands(entries)
		if len(cmds) != 1 {
			t.Fatalf("expected 1 command, got %d", len(cmds))
		}
		if len(cmds[0].Description) < 3 {
			t.Errorf("description too short for Telegram (min 3): %q", cmds[0].Description)
		}
	})

	t.Run("short description padded to minimum 3 chars", func(t *testing.T) {
		entries := []command.Entry{{Name: "x", Description: "ab"}}
		cmds := buildBotCommands(entries)
		if len(cmds) != 1 {
			t.Fatalf("expected 1 command, got %d", len(cmds))
		}
		if len(cmds[0].Description) < 3 {
			t.Errorf("description too short for Telegram (min 3): %q", cmds[0].Description)
		}
	})

	t.Run("description exactly 256 chars not truncated", func(t *testing.T) {
		exact := strings.Repeat("b", 256)
		entries := []command.Entry{{Name: "test", Description: exact}}
		cmds := buildBotCommands(entries)
		if cmds[0].Description != exact {
			t.Error("description of exactly 256 chars should not be modified")
		}
	})
}

// mockBot records Request calls for testing.
type mockBot struct {
	mu       sync.Mutex
	requests []tgbotapi.Chattable
}

func (m *mockBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return tgbotapi.Message{}, nil
}

func (m *mockBot) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, c)
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func (m *mockBot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return make(tgbotapi.UpdatesChannel)
}

func (m *mockBot) StopReceivingUpdates() {}

// stubSession implements SessionProvider for testing.
type stubSession struct{}

func (s *stubSession) CreateSession() string { return "test-session" }
func (s *stubSession) RunTurnStreamRaw(_ context.Context, _, _ string, _ func(string), _ func(string, string), _ func(string)) (string, error) {
	return "ok", nil
}
func (s *stubSession) RunSkillTurnStreamRaw(_ context.Context, _, _, _ string, _ func(string), _ func(string, string), _ func(string)) (string, error) {
	return "ok", nil
}
func (s *stubSession) WelcomeText() string { return "welcome" }

func newTestBridge(bot botAPI, registry *command.Registry) *Bridge {
	return &Bridge{
		bot:          bot,
		sm:           &stubSession{},
		commands:     registry,
		chatSessions: make(map[int64]string),
		logger:       slog.Default(),
	}
}

func TestSyncCommands(t *testing.T) {
	t.Run("sends commands at sync", func(t *testing.T) {
		reg := command.NewRegistry("")
		reg.Register(&stubCommand{name: "help", desc: "Show help"})
		reg.Register(&stubCommand{name: "status", desc: "Show status"})

		bot := &mockBot{}
		b := newTestBridge(bot, reg)

		if err := b.SyncCommands(); err != nil {
			t.Fatalf("SyncCommands error: %v", err)
		}

		bot.mu.Lock()
		defer bot.mu.Unlock()
		if len(bot.requests) != 1 {
			t.Fatalf("expected 1 request, got %d", len(bot.requests))
		}
		cfg, ok := bot.requests[0].(tgbotapi.SetMyCommandsConfig)
		if !ok {
			t.Fatalf("expected SetMyCommandsConfig, got %T", bot.requests[0])
		}
		if len(cfg.Commands) != 2 {
			t.Errorf("expected 2 commands, got %d", len(cfg.Commands))
		}
	})

	t.Run("nil registry does not error", func(t *testing.T) {
		bot := &mockBot{}
		b := newTestBridge(bot, nil)

		if err := b.SyncCommands(); err != nil {
			t.Fatalf("SyncCommands error: %v", err)
		}

		bot.mu.Lock()
		defer bot.mu.Unlock()
		if len(bot.requests) != 1 {
			t.Fatalf("expected 1 request, got %d", len(bot.requests))
		}
		cfg := bot.requests[0].(tgbotapi.SetMyCommandsConfig)
		if len(cfg.Commands) != 0 {
			t.Errorf("expected 0 commands, got %d", len(cfg.Commands))
		}
	})

	t.Run("includes skill commands", func(t *testing.T) {
		dir := t.TempDir()
		writeTestSkill(t, dir, "---\nname: deploy\ndescription: Deploy the app\n---\nDeploy instructions")

		reg := command.NewRegistry(dir)
		reg.Register(&stubCommand{name: "help", desc: "Show help"})

		bot := &mockBot{}
		b := newTestBridge(bot, reg)

		if err := b.SyncCommands(); err != nil {
			t.Fatalf("SyncCommands error: %v", err)
		}

		cfg := bot.requests[0].(tgbotapi.SetMyCommandsConfig)
		if len(cfg.Commands) != 2 {
			t.Fatalf("expected 2 commands (help + deploy), got %d", len(cfg.Commands))
		}

		names := map[string]bool{}
		for _, c := range cfg.Commands {
			names[c.Command] = true
		}
		if !names["help"] || !names["deploy"] {
			t.Errorf("expected help and deploy commands, got %v", names)
		}
	})

	t.Run("refreshes after skill added", func(t *testing.T) {
		dir := t.TempDir()
		reg := command.NewRegistry(dir)
		reg.Register(&stubCommand{name: "help", desc: "Show help"})

		bot := &mockBot{}
		b := newTestBridge(bot, reg)

		// Initial sync: only built-in "help".
		if err := b.SyncCommands(); err != nil {
			t.Fatalf("SyncCommands error: %v", err)
		}
		cfg := bot.requests[0].(tgbotapi.SetMyCommandsConfig)
		if len(cfg.Commands) != 1 {
			t.Fatalf("expected 1 command initially, got %d", len(cfg.Commands))
		}

		// Add a skill to disk.
		writeTestSkill(t, dir, "---\nname: deploy\ndescription: Deploy the app\n---\nDeploy")

		// Sync again: should now include the new skill.
		if err := b.SyncCommands(); err != nil {
			t.Fatalf("SyncCommands error: %v", err)
		}
		cfg = bot.requests[1].(tgbotapi.SetMyCommandsConfig)
		if len(cfg.Commands) != 2 {
			t.Fatalf("expected 2 commands after skill added, got %d", len(cfg.Commands))
		}
	})

	t.Run("commands are sorted alphabetically", func(t *testing.T) {
		reg := command.NewRegistry("")
		reg.Register(&stubCommand{name: "zebra", desc: "Last command"})
		reg.Register(&stubCommand{name: "alpha", desc: "First command"})

		bot := &mockBot{}
		b := newTestBridge(bot, reg)

		if err := b.SyncCommands(); err != nil {
			t.Fatalf("SyncCommands error: %v", err)
		}

		cfg := bot.requests[0].(tgbotapi.SetMyCommandsConfig)
		if len(cfg.Commands) != 2 {
			t.Fatalf("expected 2 commands, got %d", len(cfg.Commands))
		}
		if cfg.Commands[0].Command != "alpha" {
			t.Errorf("expected first command to be 'alpha', got %q", cfg.Commands[0].Command)
		}
		if cfg.Commands[1].Command != "zebra" {
			t.Errorf("expected second command to be 'zebra', got %q", cfg.Commands[1].Command)
		}
	})
}

// stubCommand is a simple Command for testing within the telegram package.
type stubCommand struct {
	name string
	desc string
}

func (c *stubCommand) Name() string        { return c.name }
func (c *stubCommand) Description() string { return c.desc }
func (c *stubCommand) Execute(_ context.Context, _ string) (string, error) {
	return "executed " + c.name, nil
}

// writeTestSkill creates a skill directory with a SKILL.md file for testing.
func writeTestSkill(t *testing.T, dir, data string) {
	t.Helper()
	s, err := skill.Parse([]byte(data))
	if err != nil {
		t.Fatalf("parse test skill: %v", err)
	}
	if err := skill.Save(dir, s); err != nil {
		t.Fatalf("save test skill: %v", err)
	}
}
