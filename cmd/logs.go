package cmd

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"wildgecu/x/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(logsCmd())
}

func logsCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show daemon logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			logPath, err := config.GlobalFilePath("wildgecu.log")
			if err != nil {
				return err
			}

			f, err := os.Open(logPath)
			if err != nil {
				return fmt.Errorf("open log file: %w", err)
			}
			defer f.Close()

			// Read last 50 lines.
			lines := readLastLines(f, 50)
			for _, line := range lines {
				fmt.Println(line)
			}

			if !follow {
				return nil
			}

			// Poll for new lines.
			scanner := bufio.NewScanner(f)
			for {
				for scanner.Scan() {
					fmt.Println(scanner.Text())
				}
				time.Sleep(500 * time.Millisecond)
			}
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	return cmd
}

func readLastLines(f *os.File, n int) []string {
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
