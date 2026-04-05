package telegram

import (
	"sort"

	"wildgecu/pkg/command"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	maxCommands      = 100
	maxDescriptionLen = 256
	minDescriptionLen = 3
)

// buildBotCommands converts command entries into Telegram BotCommands, applying
// Telegram's limits: max 100 commands and 3–256 char descriptions.
func buildBotCommands(entries []command.Entry) []tgbotapi.BotCommand {
	limit := len(entries)
	if limit > maxCommands {
		limit = maxCommands
	}

	cmds := make([]tgbotapi.BotCommand, 0, limit)
	for _, e := range entries[:limit] {
		desc := e.Description
		if len(desc) > maxDescriptionLen {
			desc = desc[:maxDescriptionLen-3] + "..."
		}
		if len(desc) < minDescriptionLen {
			desc += "..."[:minDescriptionLen-len(desc)]
		}
		cmds = append(cmds, tgbotapi.BotCommand{
			Command:     e.Name,
			Description: desc,
		})
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Command < cmds[j].Command
	})
	return cmds
}

// SyncCommands registers the current command list with Telegram's native
// autocomplete via setMyCommands.
func (b *Bridge) SyncCommands() error {
	var entries []command.Entry
	if b.commands != nil {
		entries = b.commands.List()
	}
	cmds := buildBotCommands(entries)
	cfg := tgbotapi.NewSetMyCommands(cmds...)
	_, err := b.bot.Request(cfg)
	return err
}
