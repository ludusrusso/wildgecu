package cmd

import (
	"os"

	"wildgecu/internal/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(healthCmd())
}

func healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check daemon health",
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := daemon.SendCommand("ping", nil)
			if err != nil || !resp.OK {
				os.Exit(1)
			}
			os.Exit(0)
		},
	}
}
