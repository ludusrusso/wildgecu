package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"wildgecu/pkg/command"
	"wildgecu/pkg/telegram/auth"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SessionProvider abstracts the daemon's SessionManager so that the telegram
// package does not depend on internal/daemon.
type SessionProvider interface {
	CreateSession() string // returns session ID
	RunTurnStreamRaw(ctx context.Context, id string, input string, onChunk func(string), onToolCall func(string, string), onInform func(string)) (string, error)
	RunSkillTurnStreamRaw(ctx context.Context, id, skillContent, userInput string, onChunk func(string), onToolCall func(string, string), onInform func(string)) (string, error)
	WelcomeText() string
}

// botAPI abstracts the Telegram Bot API methods used by Bridge, enabling
// testing without a real bot connection.
type botAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
}

// Bridge connects a Telegram bot to the daemon's session manager.
type Bridge struct {
	bot          botAPI
	username     string
	sm           SessionProvider
	commands     *command.Registry
	auth         *auth.Store
	chatSessions map[int64]string // chatID → session ID
	mu           sync.RWMutex
	logger       *slog.Logger
}

// New creates a new Telegram bridge using the given session provider.
// authStore may be nil to allow all users. commands may be nil to disable
// slash command handling.
func New(token string, sm SessionProvider, authStore *auth.Store, commands *command.Registry) (*Bridge, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bridge{
		bot:          bot,
		username:     bot.Self.UserName,
		sm:           sm,
		commands:     commands,
		auth:         authStore,
		chatSessions: make(map[int64]string),
		logger:       slog.Default(),
	}, nil
}

// Run starts the Telegram update loop. It blocks until ctx is cancelled.
func (b *Bridge) Run(ctx context.Context) error {
	if err := b.SyncCommands(); err != nil {
		b.logger.Warn("failed to sync telegram commands at startup", "error", err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.bot.GetUpdatesChan(u)

	b.logger.Info("telegram bot started", "username", b.username)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			if update.Message == nil {
				continue
			}
			go b.handleMessage(ctx, update.Message)
		}
	}
}

func (b *Bridge) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	// Refresh Telegram's native command autocomplete after handling, in case
	// the agent added or removed skills during this turn.
	defer func() {
		if err := b.SyncCommands(); err != nil {
			b.logger.Debug("failed to sync telegram commands", "error", err)
		}
	}()

	chatID := msg.Chat.ID

	// Auth gate: block unauthenticated users before any processing.
	if b.auth != nil && msg.From != nil && !b.auth.IsAllowed(msg.From.ID) {
		otp := b.auth.GetOrCreateOTP(msg.From.ID)
		text := fmt.Sprintf(
			"You are not authorized. Ask the owner to approve you with this code:\n\n%s\n\nThey can run: wildgecu approve telegram %s",
			otp, otp,
		)
		reply := tgbotapi.NewMessage(chatID, text)
		if _, err := b.bot.Send(reply); err != nil {
			b.logger.Error("telegram auth reply error", "error", err)
		}
		return
	}

	if msg.Text == "/start" {
		b.sendMessages(chatID, b.sm.WelcomeText())
		return
	}

	// Intercept slash commands and route to the command registry.
	if b.commands != nil && strings.HasPrefix(msg.Text, "/") {
		name, args := command.Parse(msg.Text)
		if name != "" {
			cmd := b.commands.Resolve(name)
			if cmd == nil {
				reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("Unknown command: /%s", name))
				if _, err := b.bot.Send(reply); err != nil {
					b.logger.Error("telegram send error", "error", err)
				}
				return
			}
			// Skill commands run a streaming LLM turn.
			if runner, ok := cmd.(command.SkillRunner); ok {
				b.handleSkillCommand(ctx, chatID, runner, msg.Text)
				return
			}
			// Inject session ID for session-aware commands like /clean.
			sessionID := b.getOrCreateSession(chatID)
			cmdCtx := command.WithSessionID(ctx, sessionID)
			result, err := cmd.Execute(cmdCtx, args)
			if err != nil {
				reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("Error: %v", err))
				if _, err := b.bot.Send(reply); err != nil {
					b.logger.Error("telegram send error", "error", err)
				}
				return
			}
			// If the command was /clean, update our chat→session mapping.
			if name == "clean" {
				b.updateSessionFromResult(chatID, result)
			}
			b.sendMessages(chatID, result)
			return
		}
	}

	sessionID := b.getOrCreateSession(chatID)

	// Send typing indicator
	if _, err := b.bot.Request(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)); err != nil {
		b.logger.Error("telegram send chat action error", "error", err)
	}

	h := &turnHandler{bridge: b, chatID: chatID}
	h.startTyping()
	finalContent, err := b.sm.RunTurnStreamRaw(ctx, sessionID, msg.Text, h.onChunk, h.onToolCall, h.onInform)
	h.stopTyping()
	if err != nil {
		b.logger.Error("telegram turn error", "error", err, "chat_id", chatID)
		errText := fmt.Sprintf("Error: %v", err)
		if h.msgID != 0 {
			edit := tgbotapi.NewEditMessageText(chatID, h.msgID, errText)
			if _, err := b.bot.Send(edit); err != nil {
				b.logger.Error("telegram edit error message failed", "error", err)
			}
		} else {
			if _, err := b.bot.Send(tgbotapi.NewMessage(chatID, errText)); err != nil {
				b.logger.Error("telegram send error message failed", "error", err)
			}
		}
		return
	}

	// Final edit with complete response
	if finalContent == "" {
		finalContent = "(empty response)"
	}

	// No message was created yet (no chunks arrived)
	if h.msgID == 0 {
		b.sendMessages(chatID, finalContent)
		return
	}

	// If response fits in one message, edit the existing message
	if len(finalContent) <= 4096 {
		edit := tgbotapi.NewEditMessageText(chatID, h.msgID, finalContent)
		if _, err := b.bot.Send(edit); err != nil {
			b.logger.Error("telegram final edit error", "error", err)
		}
		return
	}

	// Long response: edit with first chunk, send rest as new messages
	edit := tgbotapi.NewEditMessageText(chatID, h.msgID, finalContent[:4096])
	if _, err := b.bot.Send(edit); err != nil {
		b.logger.Error("telegram final edit error", "error", err)
	}
	b.sendMessages(chatID, finalContent[4096:])
}

func (b *Bridge) handleSkillCommand(ctx context.Context, chatID int64, runner command.SkillRunner, userInput string) {
	sessionID := b.getOrCreateSession(chatID)

	// Send typing indicator
	if _, err := b.bot.Request(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)); err != nil {
		b.logger.Error("telegram send chat action error", "error", err)
	}

	h := &turnHandler{bridge: b, chatID: chatID}
	h.startTyping()
	finalContent, err := b.sm.RunSkillTurnStreamRaw(ctx, sessionID, runner.SkillContent(), userInput, h.onChunk, h.onToolCall, h.onInform)
	h.stopTyping()
	if err != nil {
		b.logger.Error("telegram skill command error", "error", err, "chat_id", chatID)
		errText := fmt.Sprintf("Error: %v", err)
		if h.msgID != 0 {
			edit := tgbotapi.NewEditMessageText(chatID, h.msgID, errText)
			if _, err := b.bot.Send(edit); err != nil {
				b.logger.Error("telegram edit error message failed", "error", err)
			}
		} else {
			if _, err := b.bot.Send(tgbotapi.NewMessage(chatID, errText)); err != nil {
				b.logger.Error("telegram send error message failed", "error", err)
			}
		}
		return
	}

	if finalContent == "" {
		finalContent = "(empty response)"
	}

	if h.msgID == 0 {
		b.sendMessages(chatID, finalContent)
		return
	}

	if len(finalContent) <= 4096 {
		edit := tgbotapi.NewEditMessageText(chatID, h.msgID, finalContent)
		if _, err := b.bot.Send(edit); err != nil {
			b.logger.Error("telegram final edit error", "error", err)
		}
		return
	}

	edit := tgbotapi.NewEditMessageText(chatID, h.msgID, finalContent[:4096])
	if _, err := b.bot.Send(edit); err != nil {
		b.logger.Error("telegram final edit error", "error", err)
	}
	b.sendMessages(chatID, finalContent[4096:])
}

type turnHandler struct {
	bridge     *Bridge
	chatID     int64
	msgID      int // 0 until first chunk creates the message
	mu         sync.Mutex
	content    string
	lastEdit   time.Time
	typingDone chan struct{}
}

func (h *turnHandler) startTyping() {
	h.typingDone = make(chan struct{})
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-h.typingDone:
				return
			case <-ticker.C:
				if _, err := h.bridge.bot.Request(tgbotapi.NewChatAction(h.chatID, tgbotapi.ChatTyping)); err != nil {
					h.bridge.logger.Debug("telegram typing ticker error", "error", err)
				}
			}
		}
	}()
}

func (h *turnHandler) stopTyping() {
	select {
	case <-h.typingDone:
	default:
		close(h.typingDone)
	}
}

func (h *turnHandler) onChunk(chunk string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.content += chunk

	// First chunk: create the message and stop the typing indicator.
	if h.msgID == 0 && h.content != "" {
		h.stopTyping()
		text := truncate(h.content, 4000)
		msg := tgbotapi.NewMessage(h.chatID, text)
		sent, err := h.bridge.bot.Send(msg)
		if err != nil {
			h.bridge.logger.Error("telegram send first chunk error", "error", err)
			return
		}
		h.msgID = sent.MessageID
		h.lastEdit = time.Now()
		return
	}

	now := time.Now()
	if now.Sub(h.lastEdit) > time.Second && h.content != "" {
		text := truncate(h.content, 4000)
		edit := tgbotapi.NewEditMessageText(h.chatID, h.msgID, text)
		if _, err := h.bridge.bot.Send(edit); err != nil {
			h.bridge.logger.Error("telegram edit message error", "error", err)
		}
		h.lastEdit = now
	}
}

func (h *turnHandler) onToolCall(name, args string) {
	h.bridge.logger.Info("telegram tool call", "name", name, "args", args)
}

func (h *turnHandler) onInform(message string) {
	h.bridge.logger.Info("telegram inform", "message", message)
	inform := tgbotapi.NewMessage(h.chatID, "ℹ️ "+message)
	if _, err := h.bridge.bot.Send(inform); err != nil {
		h.bridge.logger.Error("telegram inform message error", "error", err)
	}
	// Refresh typing indicator as sending a message clears it
	if _, err := h.bridge.bot.Request(tgbotapi.NewChatAction(h.chatID, tgbotapi.ChatTyping)); err != nil {
		h.bridge.logger.Debug("telegram refresh chat action error", "error", err)
	}
}

// updateSessionFromResult extracts the new session ID from a /clean result
// and updates the chat→session mapping.
func (b *Bridge) updateSessionFromResult(chatID int64, result string) {
	// The result format is "Session reset. New session: <id>"
	const prefix = "New session: "
	idx := strings.Index(result, prefix)
	if idx < 0 {
		b.logger.Warn("updateSessionFromResult: could not parse new session ID from /clean result", "result", result)
		return
	}
	newID := result[idx+len(prefix):]
	b.mu.Lock()
	defer b.mu.Unlock()
	b.chatSessions[chatID] = newID
}

func (b *Bridge) getOrCreateSession(chatID int64) string {
	b.mu.RLock()
	if id, ok := b.chatSessions[chatID]; ok {
		b.mu.RUnlock()
		return id
	}
	b.mu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Double-check after acquiring write lock
	if id, ok := b.chatSessions[chatID]; ok {
		return id
	}

	id := b.sm.CreateSession()
	b.chatSessions[chatID] = id
	return id
}

// sendMessages sends text split into 4096-char chunks.
func (b *Bridge) sendMessages(chatID int64, text string) {
	if text == "" {
		return
	}
	for text != "" {
		chunk := text
		if len(chunk) > 4096 {
			chunk = text[:4096]
			text = text[4096:]
		} else {
			text = ""
		}
		msg := tgbotapi.NewMessage(chatID, chunk)
		if _, err := b.bot.Send(msg); err != nil {
			b.logger.Error("telegram send error", "error", err)
			return
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
