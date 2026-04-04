package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"wildgecu/pkg/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd())
}

type statusPayload struct {
	Watchdog map[string]int    `json:"watchdog"`
	Uptime   string            `json:"uptime"`
	Version  string            `json:"version"`
	Crons    []statusCronEntry `json:"crons,omitempty"`
	PID      int               `json:"pid"`
}

type statusCronEntry struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	NextRun  string `json:"next_run"`
	LastRun  string `json:"last_run,omitempty"`
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

			// Re-marshal/unmarshal to get typed struct
			raw, err := json.Marshal(resp.Payload)
			if err != nil {
				return err
			}
			var status statusPayload
			if err := json.Unmarshal(raw, &status); err != nil {
				return err
			}

			fmt.Printf("PID:      %d\n", status.PID)
			fmt.Printf("Uptime:   %s\n", status.Uptime)
			fmt.Printf("Version:  %s\n", status.Version)

			if len(status.Crons) == 0 {
				fmt.Println("\nNo cron jobs scheduled.")
			} else {
				fmt.Printf("\nCron jobs (%d):\n", len(status.Crons))
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "  NAME\tSCHEDULE\tNEXT RUN\tLAST RUN")
				for _, c := range status.Crons {
					lastRun := "-"
					if c.LastRun != "" {
						lastRun = c.LastRun
					}
					fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", c.Name, c.Schedule, c.NextRun, lastRun)
				}
				_ = w.Flush()
			}

			return nil
		},
	}
}
