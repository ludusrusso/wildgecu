package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/ludusrusso/wildgecu/pkg/cron"
	"github.com/ludusrusso/wildgecu/pkg/daemon"

	"github.com/spf13/cobra"
)

func init() {
	cmd := cronCmd()
	cmd.AddCommand(cronLsCmd())
	cmd.AddCommand(cronRmCmd())
	cmd.AddCommand(cronReloadCmd())
	rootCmd.AddCommand(cmd)
}

func cronsDir() (string, error) {
	h, err := newHome()
	if err != nil {
		return "", err
	}
	return h.CronsDir(), nil
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
			dir, err := cronsDir()
			if err != nil {
				return err
			}
			return runCronLs(cmd.OutOrStdout(), cmd.ErrOrStderr(), dir, daemon.IsRunning(), daemon.SendCommand)
		},
	}
}

// sendCmdFunc matches daemon.SendCommand so tests can stub the daemon round-trip.
type sendCmdFunc func(cmd string, args map[string]any) (*daemon.Response, error)

func runCronLs(stdout, stderr io.Writer, dir string, daemonUp bool, send sendCmdFunc) error {
	if daemonUp {
		resp, err := send("cron-list", nil)
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "warning: daemon query failed: %v\n", err)
		case !resp.OK:
			fmt.Fprintf(stderr, "warning: daemon query failed: %s\n", resp.Error)
		default:
			jobs, decodeErr := decodeJobInfos(resp.Payload)
			if decodeErr != nil {
				fmt.Fprintf(stderr, "warning: daemon response: %v\n", decodeErr)
			} else {
				return renderJobList(stdout, jobs)
			}
		}
	}
	return renderFilesystemFallback(stdout, stderr, dir)
}

func decodeJobInfos(payload any) ([]cron.JobInfo, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var jobs []cron.JobInfo
	if err := json.Unmarshal(raw, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func renderJobList(out io.Writer, jobs []cron.JobInfo) error {
	if len(jobs) == 0 {
		fmt.Fprintln(out, "No cron jobs found.")
		fmt.Fprintln(out, "Use 'wildgecu cron add' to create one.")
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSCHEDULE\tSTATUS\tLAST RUN\tNEXT RUN\tPROMPT")
	for _, j := range jobs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			j.Name,
			dashIfEmpty(j.Schedule),
			statusCell(j),
			formatTimeCell(j.LastRun),
			formatTimeCell(j.NextRun),
			truncPrompt(j.Prompt),
		)
	}
	return w.Flush()
}

func renderFilesystemFallback(stdout, stderr io.Writer, dir string) error {
	results := cron.LoadAllResults(dir)
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(stderr, "warning: %v\n", r.Err)
		}
	}

	if len(results) == 0 {
		fmt.Fprintln(stdout, "No cron jobs found.")
		fmt.Fprintln(stdout, "Use 'wildgecu cron add' to create one.")
		return nil
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSCHEDULE\tSTATUS\tPROMPT")
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, "-", "(daemon offline)", "")
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Job.Name, r.Job.Schedule, "(daemon offline)", truncPrompt(r.Job.Prompt))
	}
	return w.Flush()
}

func statusCell(j cron.JobInfo) string {
	if j.Status == cron.StatusError && j.Error != "" {
		return fmt.Sprintf("error: %s", j.Error)
	}
	if j.Status == "" {
		return "-"
	}
	return string(j.Status)
}

func formatTimeCell(s string) string {
	if s == "" {
		return "-"
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return s
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncPrompt(p string) string {
	if len(p) > 60 {
		return p[:57] + "..."
	}
	return p
}

func cronReloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Ask the daemon to reload cron jobs from disk",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !daemon.IsRunning() {
				return fmt.Errorf("daemon is not running")
			}
			resp, err := daemon.SendCommand("cron-reload", nil)
			if err != nil {
				return fmt.Errorf("reload: %w", err)
			}
			if !resp.OK {
				return fmt.Errorf("reload: %s", resp.Error)
			}
			fmt.Println("Cron jobs reloaded.")
			return nil
		},
	}
}

func cronRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := cronsDir()
			if err != nil {
				return err
			}

			name := args[0]
			if err := os.Remove(filepath.Join(dir, cron.Filename(name))); err != nil && !errors.Is(err, os.ErrNotExist) {
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
