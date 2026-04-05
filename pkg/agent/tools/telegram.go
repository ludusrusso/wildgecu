package tools

import (
	"context"
	"fmt"

	"wildgecu/pkg/provider/tool"
	"wildgecu/pkg/telegram/auth"
)

// TelegramTools returns tools for managing Telegram users.
// Returns nil if store is nil (Telegram auth not configured).
func TelegramTools(store *auth.Store) []tool.Tool {
	if store == nil {
		return nil
	}
	return []tool.Tool{newApproveTelegramUserTool(store)}
}

type approveInput struct {
	OTP string `json:"otp" description:"The one-time password provided by the user requesting access"`
}

type approveOutput struct {
	UserID  int64  `json:"user_id,omitempty"`
	Message string `json:"message"`
}

func newApproveTelegramUserTool(store *auth.Store) tool.Tool {
	return tool.NewTool("approve_telegram_user",
		"Approve a Telegram user by verifying their OTP code. The user will have provided this code when they first messaged the bot.",
		func(ctx context.Context, in approveInput) (approveOutput, error) {
			userID, err := store.ApproveByOTP(in.OTP)
			if err != nil {
				return approveOutput{Message: "Invalid OTP"}, nil
			}
			return approveOutput{
				UserID:  userID,
				Message: fmt.Sprintf("User %d approved successfully", userID),
			}, nil
		},
	)
}
