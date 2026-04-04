package cmd

import (
	"wildgecu/pkg/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(uninstallCmd())
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return daemon.UninstallService()
		},
	}
}
