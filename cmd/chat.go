package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gonesis/agent"
	"gonesis/homer"
	"gonesis/provider/gemini"
	"gonesis/x/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debugFlag bool

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	RunE:  runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().BoolVar(&debugFlag, "debug", false, "enable debug logging to ~/.gonesis/debug/<timestamp>.md")
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

	globalHome, err := config.GlobalHome()
	if err != nil {
		return fmt.Errorf("global home: %w", err)
	}
	home, err := homer.New(globalHome)
	if err != nil {
		return fmt.Errorf("home homer: %w", err)
	}

	workspace, err := homer.New(filepath.Join(baseDir, config.DirName))
	if err != nil {
		return fmt.Errorf("workspace homer: %w", err)
	}

	ctx := context.Background()

	p, err := gemini.New(ctx, apiKey, model)
	if err != nil {
		return err
	}

	skillsHome, err := homer.New(filepath.Join(globalHome, "skills"))
	if err != nil {
		return fmt.Errorf("skills homer: %w", err)
	}

	return agent.Run(ctx, agent.Config{
		Provider:   p,
		Home:       home,
		Workspace:  workspace,
		SkillsHome: skillsHome,
		Debug:      debugFlag,
	})
}
