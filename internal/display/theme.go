package display

import "github.com/charmbracelet/lipgloss"

// Theme defines colors for the display
// extensible: add more palettes or allow user-defined colors via config
type Theme struct {
	Repo   lipgloss.Style // repo header
	Remote lipgloss.Style // remote info, indented 2
	Output lipgloss.Style // remote output, indented 4
	Error  lipgloss.Style // error messages
}

// nord dark palette
var Nord = Theme{
	Repo:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#88C0D0")),
	Remote: lipgloss.NewStyle().Foreground(lipgloss.Color("#EBCB8B")).PaddingLeft(2),
	Output: lipgloss.NewStyle().Foreground(lipgloss.Color("#4C566A")).PaddingLeft(4),
	Error:  lipgloss.NewStyle().Foreground(lipgloss.Color("#BF616A")),
}

// catppuccin mocha
var Catppuccin = Theme{
	Repo:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89DCEB")),
	Remote: lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).PaddingLeft(2),
	Output: lipgloss.NewStyle().Foreground(lipgloss.Color("#585B70")).PaddingLeft(4),
	Error:  lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")),
}

// gruvbox dark
var Gruvbox = Theme{
	Repo:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#83A598")),
	Remote: lipgloss.NewStyle().Foreground(lipgloss.Color("#FABD2F")).PaddingLeft(2),
	Output: lipgloss.NewStyle().Foreground(lipgloss.Color("#665C54")).PaddingLeft(4),
	Error:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FB4934")),
}

// plain: no colors, suitable for piping
var Plain = Theme{
	Repo:   lipgloss.NewStyle(),
	Remote: lipgloss.NewStyle().PaddingLeft(2),
	Output: lipgloss.NewStyle().PaddingLeft(4),
	Error:  lipgloss.NewStyle(),
}

var Themes = map[string]Theme{
	"nord":       Nord,
	"catppuccin": Catppuccin,
	"gruvbox":    Gruvbox,
	"plain":      Plain,
}

// DefaultTheme is used when no theme is specified
var DefaultTheme = Nord
