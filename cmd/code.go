package cmd

import (
	"context"
	"fmt"
	"os"

	"wildgecu/pkg/chat/tui"
	"wildgecu/x/config"

	"github.com/spf13/cobra"
)

var codeCmd = &cobra.Command{
	Use:   "code",
	Short: "Start a coding agent in the current directory",
	RunE:  runCode,
}

func init() {
	rootCmd.AddCommand(codeCmd)
}

func runCode(cmd *cobra.Command, args []string) error {
	socketPath, err := config.GlobalFilePath("wildgecu.sock")
	if err != nil {
		return fmt.Errorf("resolve socket path: %w", err)
	}
	if _, statErr := os.Stat(socketPath); statErr != nil {
		return fmt.Errorf("daemon not running, start it with: wildgecu start")
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	ctx := context.Background()
	return tui.RunCode(ctx, socketPath, workDir, modelFlag)
}
