package cmd

import (
	"wildgecu/internal/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(installCmd())
}

func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install as a system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return daemon.InstallService(daemon.Config{Version: Version})
		},
	}
}
