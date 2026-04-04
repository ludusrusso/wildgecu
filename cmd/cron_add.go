package cmd

import (
	"fmt"
	"os"
	"strings"

	"wildgecu/pkg/cron"
	"wildgecu/pkg/daemon"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func init() {
	// Find the cron parent command and add the add subcommand.
	for _, c := range rootCmd.Commands() {
		if c.Use == "cron" {
			c.AddCommand(cronAddCmd())
			return
		}
	}
}

func cronAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new cron job (interactive)",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newCronAddModel()
			p := tea.NewProgram(m)
			result, err := p.Run()
			if err != nil {
				return err
			}
			final := result.(cronAddModel)
			if final.cancelled {
				fmt.Println("Cancelled.")
				return nil
			}
			return nil
		},
	}
}

type cronAddStep int

const (
	stepName cronAddStep = iota
	stepSchedule
	stepPrompt
	stepConfirm
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
)

type cronAddModel struct {
	prompt    textarea.Model
	err       error
	name      textinput.Model
	schedule  textinput.Model
	step      cronAddStep
	cancelled bool
	saved     bool
}

func newCronAddModel() cronAddModel {
	name := textinput.New()
	name.Placeholder = "daily-summary"
	name.CharLimit = 64
	name.Prompt = "> "
	name.Focus()

	schedule := textinput.New()
	schedule.Placeholder = "0 9 * * *"
	schedule.CharLimit = 64
	schedule.Prompt = "> "

	prompt := textarea.New()
	prompt.Placeholder = "Enter your LLM prompt here..."
	prompt.SetHeight(6)
	prompt.CharLimit = 0

	return cronAddModel{
		step:     stepName,
		name:     name,
		schedule: schedule,
		prompt:   prompt,
	}
}

func (m cronAddModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m cronAddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.step == stepPrompt && msg.Type == tea.KeyEsc {
				// In textarea, Esc moves to next step
				m.step = stepConfirm
				return m, nil
			}
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyEnter:
			switch m.step {
			case stepName:
				if strings.TrimSpace(m.name.Value()) == "" {
					return m, nil
				}
				m.step = stepSchedule
				m.name.Blur()
				m.schedule.Focus()
				return m, textinput.Blink

			case stepSchedule:
				if strings.TrimSpace(m.schedule.Value()) == "" {
					return m, nil
				}
				m.step = stepPrompt
				m.schedule.Blur()
				m.prompt.Focus()
				return m, textarea.Blink

			case stepConfirm:
				return m, m.save()
			}
		}

		if m.step == stepPrompt && msg.Type == tea.KeyTab {
			m.step = stepConfirm
			m.prompt.Blur()
			return m, nil
		}
	case saveResultMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.saved = true
		return m, tea.Quit
	}

	// Forward input to the active field
	var cmd tea.Cmd
	switch m.step {
	case stepName:
		m.name, cmd = m.name.Update(msg)
	case stepSchedule:
		m.schedule, cmd = m.schedule.Update(msg)
	case stepPrompt:
		m.prompt, cmd = m.prompt.Update(msg)
	}
	return m, cmd
}

type saveResultMsg struct{ err error }

func (m cronAddModel) save() tea.Cmd {
	return func() tea.Msg {
		h, err := cronsHome()
		if err != nil {
			return saveResultMsg{err: err}
		}

		job := &cron.CronJob{
			Name:     strings.TrimSpace(m.name.Value()),
			Schedule: strings.TrimSpace(m.schedule.Value()),
			Prompt:   strings.TrimSpace(m.prompt.Value()),
		}

		data, err := cron.Serialize(job)
		if err != nil {
			return saveResultMsg{err: err}
		}

		if err := h.Upsert(cron.Filename(job.Name), data); err != nil {
			return saveResultMsg{err: err}
		}

		fmt.Println(successStyle.Render(fmt.Sprintf("Cron job %q saved!", job.Name)))

		if daemon.IsRunning() {
			resp, err := daemon.SendCommand("cron-reload", nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to reload daemon: %v\n", err)
			} else if !resp.OK {
				fmt.Fprintf(os.Stderr, "warning: daemon reload failed: %s\n", resp.Error)
			}
		}

		return saveResultMsg{}
	}
}

func (m cronAddModel) View() string {
	if m.saved {
		return ""
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Add Cron Job"))
	b.WriteString("\n\n")

	// Step 1: Name
	b.WriteString(labelStyle.Render("Name:"))
	b.WriteString("\n")
	if m.step == stepName {
		b.WriteString(m.name.View())
	} else {
		b.WriteString("  " + m.name.Value())
	}
	b.WriteString("\n\n")

	if m.step < stepSchedule {
		b.WriteString(labelStyle.Render("Press Enter to continue"))
		return b.String()
	}

	// Step 2: Schedule
	b.WriteString(labelStyle.Render("Schedule (cron expression):"))
	b.WriteString("\n")
	if m.step == stepSchedule {
		b.WriteString(m.schedule.View())
	} else {
		b.WriteString("  " + m.schedule.Value())
	}
	b.WriteString("\n\n")

	if m.step < stepPrompt {
		b.WriteString(labelStyle.Render("Press Enter to continue"))
		return b.String()
	}

	// Step 3: Prompt
	b.WriteString(labelStyle.Render("Prompt:"))
	b.WriteString("\n")
	if m.step == stepPrompt {
		b.WriteString(m.prompt.View())
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Tab/Esc to continue"))
	} else {
		prompt := m.prompt.Value()
		if len(prompt) > 80 {
			prompt = prompt[:77] + "..."
		}
		b.WriteString("  " + prompt)
	}
	b.WriteString("\n\n")

	if m.step < stepConfirm {
		return b.String()
	}

	// Step 4: Confirm
	b.WriteString(titleStyle.Render("Review:"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  Name:     %s\n", m.name.Value())
	fmt.Fprintf(&b, "  Schedule: %s\n", m.schedule.Value())
	fmt.Fprintf(&b, "  Prompt:   %s\n", m.prompt.Value())
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Enter to save | Esc to cancel"))

	return b.String()
}
