package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"wildgecu/pkg/daemon"
	"wildgecu/x/config"

	"github.com/spf13/cobra"
)

func init() {
	cmd := startCmd()
	rootCmd.AddCommand(cmd)
}

func startCmd() *cobra.Command {
	var (
		system   bool
		isDaemon bool
	)
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the agent daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isDaemon {
				return runDaemon()
			}
			if system && homeFlag != "" {
				return fmt.Errorf("--home is not compatible with --system; the system service manager cannot forward custom flags")
			}
			if daemon.IsRunning() {
				return fmt.Errorf("daemon is already running")
			}
			cfg := daemon.Config{Version: Version}
			if system {
				return daemon.RunAsService(cfg)
			}
			return reExecDetached()
		},
	}
	cmd.Flags().BoolVar(&system, "system", false, "Run as a system service")
	cmd.Flags().BoolVar(&isDaemon, "daemon", false, "Run in daemon mode (internal)")
	_ = cmd.Flags().MarkHidden("daemon")
	return cmd
}

func runDaemon() error {
	logPath, err := config.GlobalFilePath("wildgecu.log")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	return daemon.Run(context.Background(), daemon.Config{
		Version:       Version,
		DefaultModel:  appConfig.DefaultModel,
		TelegramToken: appConfig.TelegramToken,
		Container:     newContainer(),
	})
}
