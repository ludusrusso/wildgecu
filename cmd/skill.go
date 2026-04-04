package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"wildgecu/x/home"
	"wildgecu/pkg/skill"
	"wildgecu/x/config"

	"github.com/spf13/cobra"
)

func init() {
	cmd := skillCmd()
	cmd.AddCommand(skillLsCmd())
	cmd.AddCommand(skillRmCmd())
	rootCmd.AddCommand(cmd)
}

func skillsHome() (home.Home, error) {
	globalHome, err := config.GlobalHome()
	if err != nil {
		return nil, err
	}
	return home.New(filepath.Join(globalHome, "skills"))
}

func skillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skill",
		Short: "Manage agent skills",
	}
}

func skillLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List all skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := skillsHome()
			if err != nil {
				return err
			}

			skills, errs := skill.LoadAll(h)
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "warning: %v\n", e)
			}

			if len(skills) == 0 {
				fmt.Println("No skills found.")
				fmt.Println("Use 'wildgecu skill add' to create one.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tDESCRIPTION\tTAGS")
			for _, s := range skills {
				tags := strings.Join(s.Tags, ", ")
				desc := s.Description
				if len(desc) > 50 {
					desc = desc[:47] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, desc, tags)
			}
			return w.Flush()
		},
	}
}

func skillRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := skillsHome()
			if err != nil {
				return err
			}

			name := args[0]
			if err := h.Delete(skill.Filename(name)); err != nil {
				return fmt.Errorf("delete skill %q: %w", name, err)
			}

			fmt.Printf("Removed skill %q\n", name)
			return nil
		},
	}
}
