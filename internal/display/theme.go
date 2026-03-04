package display

import "charm.land/lipgloss/v2"

type Theme struct {
	Repo   lipgloss.Style
	Remote lipgloss.Style
	Output lipgloss.Style
}

var Nord = Theme{
	Repo:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#88C0D0")),
	Remote: lipgloss.NewStyle().Foreground(lipgloss.Color("#EBCB8B")).PaddingLeft(2),
	Output: lipgloss.NewStyle().Foreground(lipgloss.Color("#4C566A")).PaddingLeft(4),
}

var Catppuccin = Theme{
	Repo:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89DCEB")),
	Remote: lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).PaddingLeft(2),
	Output: lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).PaddingLeft(4),
}

var Gruvbox = Theme{
	Repo:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#83A598")),
	Remote: lipgloss.NewStyle().Foreground(lipgloss.Color("#FABD2F")).PaddingLeft(2),
	Output: lipgloss.NewStyle().Foreground(lipgloss.Color("#665C54")).PaddingLeft(4),
}

var Plain = Theme{
	Repo:   lipgloss.NewStyle(),
	Remote: lipgloss.NewStyle().PaddingLeft(2),
	Output: lipgloss.NewStyle().PaddingLeft(4),
}

var Themes = map[string]Theme{
	"nord":       Nord,
	"catppuccin": Catppuccin,
	"gruvbox":    Gruvbox,
	"plain":      Plain,
}

var DefaultTheme = Nord
