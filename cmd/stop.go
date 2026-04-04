package cmd

import (
	"fmt"

	"wildgecu/pkg/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stopCmd())
}

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the agent daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := daemon.SendCommand("stop", nil)
			if err != nil {
				return err
			}
			if !resp.OK {
				return fmt.Errorf("stop failed: %s", resp.Error)
			}
			fmt.Println("Daemon stopping.")
			return nil
		},
	}
}
