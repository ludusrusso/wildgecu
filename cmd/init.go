package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"wildgecu/pkg/agent"
	"wildgecu/pkg/provider"
	"wildgecu/pkg/provider/gemini"
	"wildgecu/pkg/session"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap a new agent by creating its SOUL.md",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	apiKey := viper.GetString("gemini_api_key")
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY is not set; configure it in your config file or environment")
	}

	h, err := newHome()
	if err != nil {
		return err
	}

	// Check if SOUL.md already exists.
	existing, err := agent.LoadSoul(h)
	if err != nil {
		return err
	}
	if existing != "" {
		return fmt.Errorf("SOUL.md already exists in %s; delete it first to re-initialize", h.Dir())
	}

	ctx := context.Background()

	model := viper.GetString("model")
	var opts []gemini.Option
	if viper.GetBool("google_search") {
		opts = append(opts, gemini.WithGoogleSearch())
	}
	p, err := gemini.New(ctx, apiKey, model, opts...)
	if err != nil {
		return fmt.Errorf("gemini provider: %w", err)
	}

	var soulContent string
	cfg := agent.BootstrapConfig(ctx, p, h, &soulContent)

	// Run the initial turn (the agent speaks first).
	messages := append([]provider.Message{}, cfg.InitialMessages...)
	messages, resp, err := session.RunInitialTurn(ctx, cfg, messages)
	if err != nil && !errors.Is(err, provider.ErrDone) {
		return fmt.Errorf("initial turn: %w", err)
	}

	fmt.Println(resp.Message.Content)

	if errors.Is(err, provider.ErrDone) {
		fmt.Printf("\nSOUL.md created in %s\n", h.Dir())
		return nil
	}

	// Interactive loop.
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		messages, resp, err = session.RunTurn(ctx, cfg, messages, input)
		if err != nil && !errors.Is(err, provider.ErrDone) {
			return fmt.Errorf("turn: %w", err)
		}

		fmt.Println(resp.Message.Content)

		if errors.Is(err, provider.ErrDone) {
			fmt.Printf("\nSOUL.md created in %s\n", h.Dir())
			return nil
		}
	}

	return scanner.Err()
}
