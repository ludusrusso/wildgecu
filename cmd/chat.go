package cmd

import (
	"context"
	"fmt"
	"os"

	"gonesis/agent"
	"gonesis/provider/gemini"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	RunE:  runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

func runChat(cmd *cobra.Command, args []string) error {
	if err := ensureConfigFile(); err != nil {
		return err
	}

	apiKey := viper.GetString("gemini_api_key")
	if apiKey == "" {
		return fmt.Errorf("gemini_api_key is required: set GEMINI_API_KEY env var or add it to gonesis.yaml")
	}

	model := viper.GetString("model")

	baseDir := viper.GetString("base_folder")
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	ctx := context.Background()

	p, err := gemini.New(ctx, apiKey, model)
	if err != nil {
		return err
	}

	return agent.Run(ctx, agent.Config{
		Provider: p,
		BaseDir:  baseDir,
	})
}
