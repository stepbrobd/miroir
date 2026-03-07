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

var Catppuccin = Theme{
	Repo:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89DCEB")),
	Remote: lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")),
	Output: lipgloss.NewStyle().Foreground(lipgloss.Color("#A6ADC8")),
	Error:  lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")),
}

var Gruvbox = Theme{
	Repo:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#83A598")),
	Remote: lipgloss.NewStyle().Foreground(lipgloss.Color("#FABD2F")),
	Output: lipgloss.NewStyle().Foreground(lipgloss.Color("#BDAE93")),
	Error:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FB4934")),
}

var Plain = Theme{
	Repo:   lipgloss.NewStyle(),
	Remote: lipgloss.NewStyle(),
	Output: lipgloss.NewStyle(),
	Error:  lipgloss.NewStyle(),
}

var Themes = map[string]Theme{
	"nord":       Nord,
	"catppuccin": Catppuccin,
	"gruvbox":    Gruvbox,
	"plain":      Plain,
}

var DefaultTheme = Nord
