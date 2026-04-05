package cmd

import (
	"fmt"
	"os"

	"wildgecu/pkg/daemon"

	"github.com/spf13/cobra"
)

func init() {
	cmd := approveCmd()
	cmd.AddCommand(approveTelegramCmd())
	rootCmd.AddCommand(cmd)
}

func approveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve",
		Short: "Approve users for various integrations",
	}
}

func approveTelegramCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "telegram <OTP>",
		Short: "Approve a Telegram user by OTP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			otp := args[0]
			resp, err := daemon.SendCommand("approve-telegram", map[string]any{
				"otp": otp,
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, "Daemon not running.")
				os.Exit(2)
			}
			if !resp.OK {
				return fmt.Errorf("approval failed: %s", resp.Error)
			}
			fmt.Println(resp.Payload)
			return nil
		},
	}
}
