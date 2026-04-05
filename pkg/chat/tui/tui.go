package tui

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"wildgecu/pkg/daemon"
)

const (
	headerHeight = 2 // welcome text + blank line
	inputHeight  = 1
	statusHeight = 1
	gapLines     = 1
)

// thinkingVerbs are philosophical verbs displayed while the agent is thinking.
var thinkingVerbs = []string{
	"Prodroming",
	"Serendipiting",
	"Dialectizing",
	"Hermeneutizing",
	"Phenomenologizing",
	"Syllogizing",
	"Apophating",
	"Eudaimonizing",
	"Enteleching",
	"Catharting",
	"Dichotomizing",
	"Apodicticizing",
}

// programRef holds a pointer to the tea.Program so the streaming goroutine
// can send messages. We use a wrapper because Bubble Tea copies the Model.
type programRef struct {
	p *tea.Program
}

// Model is the Bubble Tea model for the chat TUI.
type Model struct {
	ctx            context.Context
	client         *daemon.Client
	sessionID      string
	welcomeText    string
	program        *programRef
	streamContent  string
	activeToolCall string
	display        []string
	textinput      textinput.Model
	viewport       viewport.Model
	spinner        spinner.Model
	autocomplete   *Autocomplete
	streamIdx      int
	width          int
	height         int
	thinkingIdx    int
	quitting       bool
	loading        bool
	ready          bool
	codeMode       bool
	workDir        string
}

// New creates a new TUI Model connected to the daemon via a daemon.Client.
func New(ctx context.Context, client *daemon.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 0
	ti.Prompt = "> "
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	return Model{
		ctx:          ctx,
		client:       client,
		textinput:    ti,
		spinner:      sp,
		program:      &programRef{},
		autocomplete: NewAutocomplete(nil),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.createSession)
}

// fetchCommands loads the available slash commands from the daemon.
func (m Model) fetchCommands() tea.Msg {
	cmds, err := m.client.ListCommands()
	if err != nil {
		return commandsLoadedMsg{} // silently ignore; autocomplete just won't work
	}
	return commandsLoadedMsg{commands: cmds}
}

// createSession sends session.create to the daemon.
func (m Model) createSession() tea.Msg {
	var sessionID, welcome string
	var err error
	if m.codeMode {
		sessionID, welcome, err = m.client.CreateCodeSession(m.workDir)
	} else {
		sessionID, welcome, err = m.client.CreateSession()
	}
	if err != nil {
		return sessionErrorMsg{err: err}
	}
	return sessionCreatedMsg{sessionID: sessionID, welcome: welcome}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpHeight := m.height - headerHeight - inputHeight - statusHeight - gapLines - m.acRows()
		if vpHeight < 1 {
			vpHeight = 1
		}
		if !m.ready {
			m.viewport = viewport.New(m.width, vpHeight)
			m.viewport.SetContent(strings.Join(m.display, "\n"))
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = vpHeight
		}
		m.textinput.Width = m.width - 4
		return m, nil

	case sessionCreatedMsg:
		m.sessionID = msg.sessionID
		m.welcomeText = msg.welcome
		return m, m.fetchCommands

	case commandsLoadedMsg:
		m.autocomplete = NewAutocomplete(msg.commands)
		return m, nil

	case sessionErrorMsg:
		m.appendDisplay(errorStyle.Render("Error: " + msg.err.Error()))
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			if m.sessionID != "" {
				_ = m.client.CloseSession(m.sessionID)
			}
			return m, tea.Quit
		case tea.KeyEsc:
			if m.loading && m.sessionID != "" {
				_ = m.client.InterruptSession(m.sessionID)
			}
			return m, nil
		case tea.KeyUp:
			if !m.loading && m.autocomplete.Visible() {
				m.autocomplete.MoveUp()
				return m, nil
			}
		case tea.KeyDown:
			if !m.loading && m.autocomplete.Visible() {
				m.autocomplete.MoveDown()
				return m, nil
			}
		case tea.KeyTab:
			if !m.loading && m.autocomplete.Visible() {
				if result := m.autocomplete.Complete(); result != "" {
					m.textinput.SetValue(result)
					m.textinput.CursorEnd()
					m.autocomplete.Update(result)
				}
				return m, nil
			}
		case tea.KeyEnter:
			if m.loading || m.sessionID == "" {
				return m, nil
			}
			input := strings.TrimSpace(m.textinput.Value())
			if input == "" {
				return m, nil
			}
			m.textinput.Reset()
			m.appendDisplay(lipgloss.NewStyle().Width(m.width).Render(userStyle.Render("You: ") + input))
			m.appendDisplay(assistantStyle.Render("Agent:"))
			m.appendDisplay("") // placeholder for streaming content
			m.streamIdx = len(m.display) - 1
			m.streamContent = ""
			m.loading = true
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(thinkingVerbs))))
			m.thinkingIdx = int(n.Int64())

			prog := m.program
			client := m.client
			sessionID := m.sessionID
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				if err := client.SendMessage(sessionID, input); err != nil {
					return agentErrorMsg{err: err}
				}
				// Read events in a loop until "done" or "error".
				for {
					event, err := client.ReadEvent()
					if err != nil {
						return agentErrorMsg{err: err}
					}
					switch event.Type {
					case "chunk":
						if prog.p != nil {
							prog.p.Send(streamChunkMsg{content: event.Content})
						}
					case "tool_call":
						if prog.p != nil {
							args := event.Args
							label := event.Name
							if args != "" {
								label += "(" + args + ")"
							}
							prog.p.Send(toolCallMsg{name: event.Name, args: args})
							_ = label
						}
					case "inform":
						if prog.p != nil {
							prog.p.Send(informMsg{message: event.Content})
						}
					case "done":
						return streamDoneMsg{content: event.Content}
					case "error":
						return agentErrorMsg{err: errors.New(event.Message)}
					}
				}
			})
		}

		// Forward key events to viewport for pgup/pgdn scrolling.
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.MouseMsg:
		var mouseCmd tea.Cmd
		m.viewport, mouseCmd = m.viewport.Update(msg)
		return m, mouseCmd

	case informMsg:
		m.appendDisplay(informStyle.Render(">> " + msg.message))
		return m, nil

	case toolCallMsg:
		m.activeToolCall = msg.name
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(thinkingVerbs))))
		m.thinkingIdx = int(n.Int64())
		label := msg.name
		if msg.args != "" {
			label += "(" + msg.args + ")"
		}
		m.appendDisplay(toolStyle.Render("⚡ " + label))
		return m, nil

	case streamChunkMsg:
		m.activeToolCall = ""
		m.streamContent += msg.content
		m.display[m.streamIdx] = m.renderMarkdown(m.streamContent)
		if m.ready {
			m.viewport.SetContent(strings.Join(m.display, "\n"))
			m.viewport.GotoBottom()
		}
		return m, nil

	case streamDoneMsg:
		m.loading = false
		m.activeToolCall = ""
		if msg.content != "" {
			m.display[m.streamIdx] = m.renderMarkdown(msg.content)
		}
		m.streamContent = ""
		if m.ready {
			m.viewport.SetContent(strings.Join(m.display, "\n"))
			m.viewport.GotoBottom()
		}
		return m, nil

	case agentErrorMsg:
		m.loading = false
		m.activeToolCall = ""
		if errors.Is(msg.err, context.Canceled) {
			m.appendDisplay(helpStyle.Render("Interrupted."))
			return m, nil
		}
		m.appendDisplay(lipgloss.NewStyle().Width(m.width).Render(errorStyle.Render("Error: " + msg.err.Error())))
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	if !m.loading {
		var cmd tea.Cmd
		m.textinput, cmd = m.textinput.Update(msg)
		cmds = append(cmds, cmd)
		m.autocomplete.Update(m.textinput.Value())
		m.resizeViewport()
	}

	return m, tea.Batch(cmds...)
}

const maxAutocompleteRows = 8

func (m *Model) acRows() int {
	if !m.autocomplete.Visible() {
		return 0
	}
	n := len(m.autocomplete.Matches())
	if n > maxAutocompleteRows {
		n = maxAutocompleteRows
	}
	return n
}

func (m *Model) resizeViewport() {
	if !m.ready {
		return
	}
	vpHeight := m.height - headerHeight - inputHeight - statusHeight - gapLines - m.acRows()
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.viewport.Height = vpHeight
}

func (m *Model) renderMarkdown(content string) string {
	width := m.width
	if width < 10 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content
	}
	rendered, err := r.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimRight(rendered, "\n")
}

func (m *Model) appendDisplay(line string) {
	m.display = append(m.display, line)
	if m.ready {
		m.viewport.SetContent(strings.Join(m.display, "\n"))
		m.viewport.GotoBottom()
	}
}

func (m Model) View() string {
	if !m.ready {
		return "Connecting to daemon..."
	}

	var b strings.Builder

	// Welcome header.
	if m.welcomeText != "" {
		b.WriteString(m.welcomeText)
	}
	b.WriteString("\n\n")

	// Viewport (scrollable chat).
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Status line or text input.
	switch {
	case m.sessionID == "":
		fmt.Fprintf(&b, "%s Connecting...", m.spinner.View())
	case m.loading:
		switch {
		case m.streamContent != "":
			fmt.Fprintf(&b, "%s Streaming...", m.spinner.View())
		case m.activeToolCall != "":
			fmt.Fprintf(&b, "%s %s", m.spinner.View(), toolStyle.Render("Running tool: "+m.activeToolCall+"..."))
		default:
			fmt.Fprintf(&b, "%s %s...", m.spinner.View(), thinkingVerbs[m.thinkingIdx])
		}
	default:
		b.WriteString(m.textinput.View())
	}

	// Autocomplete dropdown.
	if !m.loading && m.autocomplete.Visible() {
		b.WriteString("\n")
		for i, cmd := range m.autocomplete.Matches() {
			if i >= maxAutocompleteRows {
				break
			}
			if i == m.autocomplete.Selected() {
				b.WriteString(acSelectedStyle.Render("  /" + cmd.Name + " — " + cmd.Description))
			} else {
				b.WriteString(acNormalStyle.Render("  /" + cmd.Name + " — " + cmd.Description))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.loading {
		b.WriteString(helpStyle.Render("Esc: interrupt | Ctrl+C: quit"))
	} else {
		b.WriteString(helpStyle.Render("Enter: send | Ctrl+C: quit"))
	}

	return b.String()
}

// Run creates a Model and runs the Bubble Tea program.
func Run(ctx context.Context, socketPath string) error {
	client, err := daemon.Connect(socketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	m := New(ctx, client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program.p = p
	_, err = p.Run()
	return err
}

// RunCode creates a code-mode Model and runs the Bubble Tea program.
func RunCode(ctx context.Context, socketPath, workDir string) error {
	client, err := daemon.Connect(socketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	m := New(ctx, client)
	m.codeMode = true
	m.workDir = workDir
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program.p = p
	_, err = p.Run()
	return err
}
