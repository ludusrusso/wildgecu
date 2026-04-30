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

	"github.com/ludusrusso/wildgecu/pkg/agent/tools"
	"github.com/ludusrusso/wildgecu/pkg/daemon"
	"github.com/ludusrusso/wildgecu/pkg/todo"
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
	ctx                 context.Context
	client              *daemon.Client
	sessionID           string
	welcomeText         string
	program             *programRef
	streamContent       string
	activeToolCall      string
	display             []string
	toolCallsThisTurn   int
	toolCallSlotIdxs    []int
	toolCallOverflowIdx int
	textinput           textinput.Model
	viewport            viewport.Model
	spinner             spinner.Model
	autocomplete        *Autocomplete
	streamIdx           int
	width               int
	height              int
	thinkingIdx         int
	quitting            bool
	loading             bool
	ready               bool
	codeMode            bool
	workDir             string
	model               string
	todos               []todo.Item
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
		ctx:                 ctx,
		client:              client,
		textinput:           ti,
		spinner:             sp,
		program:             &programRef{},
		autocomplete:        NewAutocomplete(nil),
		toolCallOverflowIdx: -1,
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
		sessionID, welcome, err = m.client.CreateCodeSession(m.workDir, m.model)
	} else {
		sessionID, welcome, err = m.client.CreateSession(m.model)
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
		vpHeight := m.height - headerHeight - inputHeight - statusHeight - gapLines - m.acRows() - m.todoRows()
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
			m.toolCallsThisTurn = 0
			m.resizeViewport()
			m.toolCallSlotIdxs = nil
			m.toolCallOverflowIdx = -1
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
							prog.p.Send(toolCallMsg{name: event.Name, args: event.Args, agent: event.Agent})
						}
					case "inform":
						if prog.p != nil {
							prog.p.Send(informMsg{message: event.Content})
						}
					case "todo_snapshot":
						if prog.p != nil {
							prog.p.Send(todoSnapshotMsg{items: event.Todos})
						}
					case "done":
						return streamDoneMsg{content: event.Content, sessionID: event.SessionID}
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
		if m.loading && m.streamIdx != -1 {
			m.insertDisplayBeforeStream(informStyle.Render(">> " + msg.message))
		} else {
			m.appendDisplay(informStyle.Render(">> " + msg.message))
		}
		return m, nil

	case toolCallMsg:
		m.activeToolCall = msg.name
		if msg.agent != "" {
			m.activeToolCall = msg.name + " (" + msg.agent + ")"
		}
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(thinkingVerbs))))
		m.thinkingIdx = int(n.Int64())
		label := formatToolCallLabel(msg.name, msg.args)
		if msg.agent != "" {
			label = "[" + msg.agent + "] " + label
		}
		rendered := toolStyle.Render("⚡ ") + toolDimStyle.Render(label)
		m.resizeViewport()
		m.toolCallsThisTurn++
		if m.toolCallsThisTurn <= maxToolCallsPerTurn {
			idx := m.insertDisplayBeforeStream(rendered)
			m.toolCallSlotIdxs = append(m.toolCallSlotIdxs, idx)
		} else {
			summary := toolStyle.Render(fmt.Sprintf("+%d more tool calls", m.toolCallsThisTurn-maxToolCallsPerTurn))
			if m.toolCallOverflowIdx == -1 {
				m.toolCallOverflowIdx = m.toolCallSlotIdxs[0]
				m.display[m.toolCallOverflowIdx] = summary
				idx := m.insertDisplayBeforeStream(rendered)
				m.toolCallSlotIdxs = append(m.toolCallSlotIdxs[1:], idx)
			} else {
				for i := 0; i < len(m.toolCallSlotIdxs)-1; i++ {
					m.display[m.toolCallSlotIdxs[i]] = m.display[m.toolCallSlotIdxs[i+1]]
				}
				m.display[m.toolCallSlotIdxs[len(m.toolCallSlotIdxs)-1]] = rendered
				m.display[m.toolCallOverflowIdx] = summary
			}
			if m.ready {
				m.viewport.SetContent(strings.Join(m.display, "\n"))
				if m.viewport.AtBottom() {
					m.viewport.GotoBottom()
				}
			}
		}
		return m, nil

	case todoSnapshotMsg:
		m.todos = msg.items
		m.resizeViewport()
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
		if msg.sessionID != "" {
			if msg.sessionID != m.sessionID {
				m.todos = nil
				m.resizeViewport()
			}
			m.sessionID = msg.sessionID
		}
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
const maxToolCallsPerTurn = 4

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
	vpHeight := m.height - headerHeight - inputHeight - statusHeight - gapLines - m.acRows() - m.todoRows()
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.viewport.Height = vpHeight
}

func (m *Model) todoRows() int {
	rendered := m.renderTodos()
	if rendered == "" {
		return 0
	}
	return lipgloss.Height(rendered) + 2 // 2 newlines + content
}

// renderTodos returns the sticky region's string representation, or empty
// when there are no todos.
func (m *Model) renderTodos() string {
	if len(m.todos) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(todoHeaderStyle.Render("📋 TODO LIST") + "\n")

	for _, it := range m.todos {
		checkbox := todoCheckbox(it.Status)
		prefix := fmt.Sprintf(" %s ", checkbox)
		prefixLen := lipgloss.Width(prefix)

		contentWidth := m.width - prefixLen
		if contentWidth < 20 {
			contentWidth = 20
		}

		content := lipgloss.NewStyle().
			Width(contentWidth).
			Render(it.Content)

		item := lipgloss.JoinHorizontal(lipgloss.Top, prefix, content)
		b.WriteString(todoStyle.Render(item) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func todoCheckbox(s todo.Status) string {
	switch s {
	case todo.StatusCompleted:
		return "[x]"
	case todo.StatusInProgress:
		return "[~]"
	case todo.StatusCancelled:
		return "[-]"
	default:
		return "[ ]"
	}
}

var argEscaper = strings.NewReplacer("\n", "\\n", "\r", "\\r")

// formatToolCallLabel returns a compact inline label, using specialized
// summaries for todo tool calls instead of raw-args dumps.
func formatToolCallLabel(name, args string) string {
	switch name {
	case tools.TodoCreateName:
		return name + "(" + summarizeTodoCreate(args) + ")"
	case tools.TodoUpdateName:
		return name + "(" + summarizeTodoUpdate(args) + ")"
	}
	if args == "" {
		return name
	}
	return name + "(" + argEscaper.Replace(args) + ")"
}

// summarizeTodoCreate parses "contents: [a b c]" from provider.FormatToolArgs
// and returns an "N items" / "1 item" summary.
func summarizeTodoCreate(args string) string {
	openIdx := strings.Index(args, "[")
	closeIdx := strings.LastIndex(args, "]")
	if openIdx < 0 || closeIdx <= openIdx {
		return "items"
	}
	inner := strings.TrimSpace(args[openIdx+1 : closeIdx])
	if inner == "" {
		return "0 items"
	}
	n := len(strings.Fields(inner))
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}

// summarizeTodoUpdate pulls id and status out of "id: 2, status: completed, ..."
// and renders "#2 → completed".
func summarizeTodoUpdate(args string) string {
	id := extractArg(args, "id")
	status := extractArg(args, "status")
	switch {
	case id != "" && status != "":
		return "#" + id + " → " + status
	case id != "":
		return "#" + id
	case status != "":
		return "→ " + status
	default:
		return args
	}
}

func extractArg(args, key string) string {
	prefix := key + ": "
	i := strings.Index(args, prefix)
	if i < 0 {
		return ""
	}
	rest := args[i+len(prefix):]
	if j := strings.Index(rest, ","); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
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
		if m.viewport.AtBottom() {
			m.viewport.GotoBottom()
		}
	}
}

func (m *Model) insertDisplayBeforeStream(line string) int {
	idx := m.streamIdx
	if idx < 0 || idx >= len(m.display) {
		m.appendDisplay(line)
		return len(m.display) - 1
	}

	// Efficient insert
	m.display = append(m.display, "")
	copy(m.display[idx+1:], m.display[idx:])
	m.display[idx] = line

	m.streamIdx++

	if m.ready {
		m.viewport.SetContent(strings.Join(m.display, "\n"))
		if m.viewport.AtBottom() {
			m.viewport.GotoBottom()
		}
	}
	return idx
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

	// Sticky regions

	if rendered := m.renderTodos(); rendered != "" {
		b.WriteString("\n")
		b.WriteString(rendered)
		b.WriteString("\n")
	}

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
func Run(ctx context.Context, socketPath, model string) error {
	client, err := daemon.Connect(socketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	m := New(ctx, client)
	m.model = model
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program.p = p
	_, err = p.Run()
	return err
}

// RunCode creates a code-mode Model and runs the Bubble Tea program.
func RunCode(ctx context.Context, socketPath, workDir, model string) error {
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
	m.model = model
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program.p = p
	_, err = p.Run()
	return err
}
