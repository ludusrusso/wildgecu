package tui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"gonesis/chat"
	"gonesis/provider"
)

const (
	headerHeight = 2 // welcome text + blank line
	inputHeight  = 1
	statusHeight = 1
	gapLines     = 1
)

// programRef holds a pointer to the tea.Program so the streaming goroutine
// can send messages. We use a wrapper because Bubble Tea copies the Model.
type programRef struct {
	p *tea.Program
}

// Model is the Bubble Tea model for the chat TUI.
type Model struct {
	ctx           context.Context
	err           error
	cfg           *chat.Config
	program       *programRef
	streamContent  string
	activeToolCall string
	messages       []provider.Message
	display       []string
	textinput     textinput.Model
	viewport      viewport.Model
	spinner       spinner.Model
	streamIdx     int
	width         int
	height        int
	done          bool
	quitting      bool
	loading       bool
	ready         bool
}

// New creates a new TUI Model.
func New(ctx context.Context, cfg *chat.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 0
	ti.Prompt = "> "
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	return Model{
		cfg:       cfg,
		ctx:       ctx,
		textinput: ti,
		spinner:   sp,
		messages:  append([]provider.Message{}, cfg.InitialMessages...),
		program:   &programRef{},
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	if len(m.cfg.InitialMessages) > 0 {
		cmds = append(cmds, func() tea.Msg { return initialTurnMsg{} })
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpHeight := m.height - headerHeight - inputHeight - statusHeight - gapLines
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

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			if m.cfg.OnDone != nil {
				m.cfg.OnDone(m.messages)
			}
			return m, tea.Quit
		case tea.KeyEnter:
			if m.loading {
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
			// Copy messages for concurrency safety.
			msgs := make([]provider.Message, len(m.messages))
			copy(msgs, m.messages)
			cfg := m.cfg
			ctx := m.ctx
			prog := m.program
			cfg.OnToolCall = func(tc provider.ToolCall) {
				if prog.p != nil {
					prog.p.Send(toolCallMsg{name: tc.Name, args: formatToolArgs(tc.Args, 100)})
				}
			}
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				onChunk := func(chunk string) {
					if prog.p != nil {
						prog.p.Send(streamChunkMsg{content: chunk})
					}
				}
				updated, resp, err := chat.RunTurnStream(ctx, cfg, msgs, input, onChunk)
				if err != nil {
					if errors.Is(err, provider.ErrDone) {
						return agentDoneMsg{messages: updated}
					}
					return agentErrorMsg{err: err}
				}
				return streamDoneMsg{messages: updated, response: resp}
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

	case initialTurnMsg:
		m.appendDisplay(assistantStyle.Render("Agent:"))
		m.appendDisplay("") // placeholder for streaming content
		m.streamIdx = len(m.display) - 1
		m.streamContent = ""
		m.loading = true
		msgs := make([]provider.Message, len(m.messages))
		copy(msgs, m.messages)
		cfg := m.cfg
		ctx := m.ctx
		prog := m.program
		cfg.OnToolCall = func(tc provider.ToolCall) {
			if prog.p != nil {
				prog.p.Send(toolCallMsg{name: tc.Name, args: formatToolArgs(tc.Args, 100)})
			}
		}
		return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
			onChunk := func(chunk string) {
				if prog.p != nil {
					prog.p.Send(streamChunkMsg{content: chunk})
				}
			}
			updated, resp, err := chat.RunInitialTurnStream(ctx, cfg, msgs, onChunk)
			if err != nil {
				if errors.Is(err, provider.ErrDone) {
					return agentDoneMsg{messages: updated}
				}
				return agentErrorMsg{err: err}
			}
			return streamDoneMsg{messages: updated, response: resp}
		})

	case agentResponseMsg:
		m.messages = msg.messages
		m.loading = false
		if msg.response != nil && msg.response.Message.Content != "" {
			m.appendDisplay(assistantStyle.Render("Agent:"))
			m.appendDisplay(m.renderMarkdown(msg.response.Message.Content))
		}
		return m, nil

	case toolCallMsg:
		m.activeToolCall = msg.name
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
		m.messages = msg.messages
		m.loading = false
		m.activeToolCall = ""
		if msg.response != nil && msg.response.Message.Content != "" {
			m.display[m.streamIdx] = m.renderMarkdown(msg.response.Message.Content)
		}
		m.streamContent = ""
		if m.ready {
			m.viewport.SetContent(strings.Join(m.display, "\n"))
			m.viewport.GotoBottom()
		}
		return m, nil

	case agentDoneMsg:
		m.messages = msg.messages
		m.loading = false
		m.activeToolCall = ""
		m.done = true
		if m.cfg.OnDone != nil {
			m.cfg.OnDone(msg.messages)
		}
		return m, tea.Quit

	case agentErrorMsg:
		m.loading = false
		m.activeToolCall = ""
		m.err = msg.err
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
	}

	return m, tea.Batch(cmds...)
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
		return "Initializing..."
	}

	var b strings.Builder

	// Welcome header.
	if m.cfg.WelcomeText != "" {
		b.WriteString(m.cfg.WelcomeText)
	}
	b.WriteString("\n\n")

	// Viewport (scrollable chat).
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Status line or text input.
	if m.loading {
		if m.streamContent != "" {
			fmt.Fprintf(&b, "%s Streaming...", m.spinner.View())
		} else if m.activeToolCall != "" {
			fmt.Fprintf(&b, "%s %s", m.spinner.View(), toolStyle.Render("Running tool: "+m.activeToolCall+"..."))
		} else {
			fmt.Fprintf(&b, "%s Thinking...", m.spinner.View())
		}
	} else {
		b.WriteString(m.textinput.View())
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter: send | Ctrl+C: quit"))

	return b.String()
}

// formatToolArgs formats a tool call's args map into a compact string,
// truncated to maxLen characters.
func formatToolArgs(args map[string]any, maxLen int) string {
	if len(args) == 0 {
		return ""
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		v := fmt.Sprintf("%v", args[k])
		parts = append(parts, k+": "+v)
	}
	result := strings.Join(parts, ", ")
	if len(result) > maxLen {
		result = result[:maxLen] + "..."
	}
	return result
}

// Run creates a Model and runs the Bubble Tea program.
func Run(ctx context.Context, cfg *chat.Config) error {
	m := New(ctx, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program.p = p
	_, err := p.Run()
	return err
}
