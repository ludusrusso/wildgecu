package cmd

import (
	"fmt"

	"wildgecu/internal/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(updateCmd())
}

func updateCmd() *cobra.Command {
	var url string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Trigger a self-update of the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if url == "" {
				return fmt.Errorf("--url is required")
			}
			resp, err := daemon.SendCommand("update", map[string]any{"url": url})
			if err != nil {
				return err
			}
			if !resp.OK {
				return fmt.Errorf("update failed: %s", resp.Error)
			}
			fmt.Println("Update started.")
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "URL of the new binary")
	return cmd
}
