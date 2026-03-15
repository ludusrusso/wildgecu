package cmd

import (
	"fmt"
	"time"

	"wildgecu/internal/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(restartCmd())
}

func restartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the agent daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemon.IsRunning() {
				resp, err := daemon.SendCommand("stop", nil)
				if err != nil {
					return err
				}
				if !resp.OK {
					return fmt.Errorf("stop failed: %s", resp.Error)
				}
				// Wait for the daemon to exit.
				for i := 0; i < 20; i++ {
					time.Sleep(250 * time.Millisecond)
					if !daemon.IsRunning() {
						break
					}
				}
			}
			return reExecDetached()
		},
	}
}
