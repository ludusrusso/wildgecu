package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"wildgecu/pkg/telegram/auth"
)

// SessionProvider abstracts the daemon's SessionManager so that the telegram
// package does not depend on internal/daemon.
type SessionProvider interface {
	CreateSession() string // returns session ID
	RunTurnStreamRaw(ctx context.Context, id string, input string, onChunk func(string), onToolCall func(string, string), onInform func(string)) (string, error)
	WelcomeText() string
}

// Bridge connects a Telegram bot to the daemon's session manager.
type Bridge struct {
	bot          *tgbotapi.BotAPI
	sm           SessionProvider
	auth         *auth.Store
	chatSessions map[int64]string // chatID → session ID
	mu           sync.RWMutex
	logger       *slog.Logger
}

// New creates a new Telegram bridge using the given session provider.
// authStore may be nil to allow all users.
func New(token string, sm SessionProvider, authStore *auth.Store) (*Bridge, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bridge{
		bot:          bot,
		sm:           sm,
		auth:         authStore,
		chatSessions: make(map[int64]string),
		logger:       slog.Default(),
	}, nil
}

// Run starts the Telegram update loop. It blocks until ctx is cancelled.
func (b *Bridge) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.bot.GetUpdatesChan(u)

	b.logger.Info("telegram bot started", "username", b.bot.Self.UserName)

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

	sessionID := b.getOrCreateSession(chatID)

	// Send typing indicator
	if _, err := b.bot.Request(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)); err != nil {
		b.logger.Error("telegram send chat action error", "error", err)
	}

	// Send placeholder message
	placeholder := tgbotapi.NewMessage(chatID, "...")
	sent, err := b.bot.Send(placeholder)
	if err != nil {
		b.logger.Error("telegram send placeholder error", "error", err)
		return
	}

	// Stream with throttled edits
	var mu sync.Mutex
	var content string
	lastEdit := time.Time{}

	onChunk := func(chunk string) {
		mu.Lock()
		defer mu.Unlock()
		content += chunk
		now := time.Now()
		if now.Sub(lastEdit) > time.Second && content != "" {
			text := truncate(content, 4000)
			edit := tgbotapi.NewEditMessageText(chatID, sent.MessageID, text)
			if _, editErr := b.bot.Send(edit); editErr != nil {
				b.logger.Error("telegram edit message error", "error", editErr)
			}
			lastEdit = now
		}
	}

	onToolCall := func(name, args string) {
		b.logger.Info("telegram tool call", "name", name, "args", args)
	}

	onInform := func(message string) {
		b.logger.Info("telegram inform", "message", message)
		inform := tgbotapi.NewMessage(chatID, "ℹ️ "+message)
		if _, err := b.bot.Send(inform); err != nil {
			b.logger.Error("telegram inform message error", "error", err)
		}
		// Refresh typing indicator as sending a message clears it
		if _, err := b.bot.Request(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)); err != nil {
			b.logger.Debug("telegram refresh chat action error", "error", err)
		}
	}

	finalContent, err := b.sm.RunTurnStreamRaw(ctx, sessionID, msg.Text, onChunk, onToolCall, onInform)
	if err != nil {
		b.logger.Error("telegram turn error", "error", err, "chat_id", chatID)
		edit := tgbotapi.NewEditMessageText(chatID, sent.MessageID, fmt.Sprintf("Error: %v", err))
		if _, err := b.bot.Send(edit); err != nil {
			b.logger.Error("telegram edit error message failed", "error", err)
		}
		return
	}

	// Final edit with complete response
	if finalContent == "" {
		finalContent = "(empty response)"
	}

	// If response fits in one message, edit the placeholder
	if len(finalContent) <= 4096 {
		edit := tgbotapi.NewEditMessageText(chatID, sent.MessageID, finalContent)
		if _, err := b.bot.Send(edit); err != nil {
			b.logger.Error("telegram final edit error", "error", err)
		}
		return
	}

	// Long response: edit placeholder with first chunk, send rest as new messages
	edit := tgbotapi.NewEditMessageText(chatID, sent.MessageID, finalContent[:4096])
	if _, err := b.bot.Send(edit); err != nil {
		b.logger.Error("telegram final edit error", "error", err)
	}
	b.sendMessages(chatID, finalContent[4096:])
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
