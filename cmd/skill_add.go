package cmd

import (
	"fmt"
	"strings"

	"wildgecu/skill"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func init() {
	for _, c := range rootCmd.Commands() {
		if c.Use == "skill" {
			c.AddCommand(skillAddCmd())
			return
		}
	}
}

func skillAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new skill (interactive)",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newSkillAddModel()
			p := tea.NewProgram(m)
			result, err := p.Run()
			if err != nil {
				return err
			}
			final := result.(skillAddModel)
			if final.cancelled {
				fmt.Println("Cancelled.")
				return nil
			}
			return nil
		},
	}
}

type skillAddStep int

const (
	skillStepName skillAddStep = iota
	skillStepDescription
	skillStepTags
	skillStepContent
	skillStepConfirm
)

type skillAddModel struct {
	content     textarea.Model
	err         error
	name        textinput.Model
	description textinput.Model
	tags        textinput.Model
	step        skillAddStep
	cancelled   bool
	saved       bool
}

func newSkillAddModel() skillAddModel {
	name := textinput.New()
	name.Placeholder = "go-error-handling"
	name.CharLimit = 64
	name.Prompt = "> "
	name.Focus()

	description := textinput.New()
	description.Placeholder = "Best practices for Go error handling"
	description.CharLimit = 256
	description.Prompt = "> "

	tags := textinput.New()
	tags.Placeholder = "go, errors (comma-separated)"
	tags.CharLimit = 256
	tags.Prompt = "> "

	content := textarea.New()
	content.Placeholder = "Enter your skill content as markdown..."
	content.SetHeight(10)
	content.CharLimit = 0

	return skillAddModel{
		step:        skillStepName,
		name:        name,
		description: description,
		tags:        tags,
		content:     content,
	}
}

func (m skillAddModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m skillAddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.step == skillStepContent && msg.Type == tea.KeyEsc {
				m.step = skillStepConfirm
				return m, nil
			}
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyEnter:
			switch m.step {
			case skillStepName:
				if strings.TrimSpace(m.name.Value()) == "" {
					return m, nil
				}
				m.step = skillStepDescription
				m.name.Blur()
				m.description.Focus()
				return m, textinput.Blink

			case skillStepDescription:
				if strings.TrimSpace(m.description.Value()) == "" {
					return m, nil
				}
				m.step = skillStepTags
				m.description.Blur()
				m.tags.Focus()
				return m, textinput.Blink

			case skillStepTags:
				m.step = skillStepContent
				m.tags.Blur()
				m.content.Focus()
				return m, textarea.Blink

			case skillStepConfirm:
				return m, m.save()
			}
		}

		if m.step == skillStepContent && msg.Type == tea.KeyTab {
			m.step = skillStepConfirm
			m.content.Blur()
			return m, nil
		}
	case skillSaveResultMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.saved = true
		return m, tea.Quit
	}

	var cmd tea.Cmd
	switch m.step {
	case skillStepName:
		m.name, cmd = m.name.Update(msg)
	case skillStepDescription:
		m.description, cmd = m.description.Update(msg)
	case skillStepTags:
		m.tags, cmd = m.tags.Update(msg)
	case skillStepContent:
		m.content, cmd = m.content.Update(msg)
	}
	return m, cmd
}

type skillSaveResultMsg struct{ err error }

func (m skillAddModel) parseTags() []string {
	raw := strings.TrimSpace(m.tags.Value())
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func (m skillAddModel) save() tea.Cmd {
	return func() tea.Msg {
		h, err := skillsHomer()
		if err != nil {
			return skillSaveResultMsg{err: err}
		}

		s := &skill.Skill{
			Name:        strings.TrimSpace(m.name.Value()),
			Description: strings.TrimSpace(m.description.Value()),
			Tags:        m.parseTags(),
			Content:     strings.TrimSpace(m.content.Value()),
		}

		data, err := skill.Serialize(s)
		if err != nil {
			return skillSaveResultMsg{err: err}
		}

		if err := h.Upsert(skill.Filename(s.Name), data); err != nil {
			return skillSaveResultMsg{err: err}
		}

		fmt.Println(successStyle.Render(fmt.Sprintf("Skill %q saved!", s.Name)))

		return skillSaveResultMsg{}
	}
}

func (m skillAddModel) View() string {
	if m.saved {
		return ""
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Add Skill"))
	b.WriteString("\n\n")

	// Step 1: Name
	b.WriteString(labelStyle.Render("Name:"))
	b.WriteString("\n")
	if m.step == skillStepName {
		b.WriteString(m.name.View())
	} else {
		b.WriteString("  " + m.name.Value())
	}
	b.WriteString("\n\n")

	if m.step < skillStepDescription {
		b.WriteString(labelStyle.Render("Press Enter to continue"))
		return b.String()
	}

	// Step 2: Description
	b.WriteString(labelStyle.Render("Description:"))
	b.WriteString("\n")
	if m.step == skillStepDescription {
		b.WriteString(m.description.View())
	} else {
		b.WriteString("  " + m.description.Value())
	}
	b.WriteString("\n\n")

	if m.step < skillStepTags {
		b.WriteString(labelStyle.Render("Press Enter to continue"))
		return b.String()
	}

	// Step 3: Tags
	b.WriteString(labelStyle.Render("Tags (comma-separated, optional):"))
	b.WriteString("\n")
	if m.step == skillStepTags {
		b.WriteString(m.tags.View())
	} else {
		b.WriteString("  " + m.tags.Value())
	}
	b.WriteString("\n\n")

	if m.step < skillStepContent {
		b.WriteString(labelStyle.Render("Press Enter to continue"))
		return b.String()
	}

	// Step 4: Content
	b.WriteString(labelStyle.Render("Content (markdown):"))
	b.WriteString("\n")
	if m.step == skillStepContent {
		b.WriteString(m.content.View())
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Tab/Esc to continue"))
	} else {
		content := m.content.Value()
		if len(content) > 80 {
			content = content[:77] + "..."
		}
		b.WriteString("  " + content)
	}
	b.WriteString("\n\n")

	if m.step < skillStepConfirm {
		return b.String()
	}

	// Step 5: Confirm
	b.WriteString(titleStyle.Render("Review:"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  Name:        %s\n", m.name.Value())
	fmt.Fprintf(&b, "  Description: %s\n", m.description.Value())
	fmt.Fprintf(&b, "  Tags:        %s\n", m.tags.Value())
	fmt.Fprintf(&b, "  Content:     %s\n", m.content.Value())
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Enter to save | Esc to cancel"))

	return b.String()
}
