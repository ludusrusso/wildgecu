package cmd

import (
	"context"
	"fmt"
	"os"

	"wildgecu/pkg/chat/tui"
	"wildgecu/x/config"

	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session (connects to daemon)",
	RunE:  runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

func runChat(cmd *cobra.Command, args []string) error {
	socketPath, err := config.GlobalFilePath("wildgecu.sock")
	if err != nil {
		return fmt.Errorf("resolve socket path: %w", err)
	}
	if _, err := os.Stat(socketPath); err != nil {
		return fmt.Errorf("daemon not running, start it with: wildgecu start")
	}

	ctx := context.Background()
	return tui.Run(ctx, socketPath, modelFlag)
}
