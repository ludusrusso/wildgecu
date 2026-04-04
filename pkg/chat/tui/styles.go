package tui

import "github.com/charmbracelet/lipgloss"

var (
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	helpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	spinnerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)
