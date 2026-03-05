package formoverlay

import "github.com/charmbracelet/lipgloss"

const (
	defaultMaxWidth  = 80
	defaultMaxHeight = 30
)

// DefaultBorderStyle returns the default modal border style.
func DefaultBorderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)
}

// DefaultTitleStyle returns the default title bar style.
func DefaultTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("62")).
		Padding(0, 1).
		MarginBottom(1)
}
