package display

import "charm.land/lipgloss/v2"

type Theme struct {
	Repo   lipgloss.Style
	Remote lipgloss.Style
	Output lipgloss.Style
	Error  lipgloss.Style
}

// padding is applied centrally in display.styled(), not in theme definitions

var Nord = Theme{
	Repo:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#88C0D0")),
	Remote: lipgloss.NewStyle().Foreground(lipgloss.Color("#EBCB8B")),
	Output: lipgloss.NewStyle().Foreground(lipgloss.Color("#D8DEE9")),
	Error:  lipgloss.NewStyle().Foreground(lipgloss.Color("#BF616A")),
}

var DefaultTheme = Nord
