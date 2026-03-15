package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"wildgecu/cron"
	"wildgecu/homer"
	"wildgecu/internal/daemon"
	"wildgecu/x/config"

	"github.com/spf13/cobra"
)

func init() {
	cmd := cronCmd()
	cmd.AddCommand(cronLsCmd())
	cmd.AddCommand(cronRmCmd())
	rootCmd.AddCommand(cmd)
}

func cronsHomer() (homer.Homer, error) {
	globalHome, err := config.GlobalHome()
	if err != nil {
		return nil, err
	}
	return homer.New(filepath.Join(globalHome, "crons"))
}

func cronCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cron",
		Short: "Manage scheduled cron jobs",
	}
}

func cronLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List all cron jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := cronsHomer()
			if err != nil {
				return err
			}

			jobs, errs := cron.LoadAll(h)
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "warning: %v\n", e)
			}

			if len(jobs) == 0 {
				fmt.Println("No cron jobs found.")
				fmt.Println("Use 'wildgecu cron add' to create one.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSCHEDULE\tPROMPT")
			for _, j := range jobs {
				prompt := j.Prompt
				if len(prompt) > 60 {
					prompt = prompt[:57] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", j.Name, j.Schedule, prompt)
			}
			return w.Flush()
		},
	}
}

func cronRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := cronsHomer()
			if err != nil {
				return err
			}

			name := args[0]
			if err := h.Delete(cron.Filename(name)); err != nil {
				return fmt.Errorf("delete cron job %q: %w", name, err)
			}

			fmt.Printf("Removed cron job %q\n", name)

			if daemon.IsRunning() {
				resp, err := daemon.SendCommand("cron-reload", nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to reload daemon: %v\n", err)
				} else if !resp.OK {
					fmt.Fprintf(os.Stderr, "warning: daemon reload failed: %s\n", resp.Error)
				}
			}

			return nil
		},
	}
}
