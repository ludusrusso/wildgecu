package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"gonesis/internal/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd())
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := daemon.SendCommand("status", nil)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Daemon not running.")
				os.Exit(2)
			}
			if !resp.OK {
				return fmt.Errorf("status error: %s", resp.Error)
			}

			data, err := json.MarshalIndent(resp.Payload, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}
}
