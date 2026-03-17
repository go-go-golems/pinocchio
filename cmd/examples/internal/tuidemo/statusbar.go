package tuidemo

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type StatusPart struct {
	Label string
	Value string
}

func NewStatusBarView(parts []StatusPart, hint string) func() string {
	barStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.Color("236")).
		Padding(0, 1)
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Background(lipgloss.Color("236"))
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("236")).
		Bold(true)
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("236")).
		Italic(true)

	return func() string {
		segments := make([]string, 0, len(parts)+1)
		for _, part := range parts {
			if strings.TrimSpace(part.Label) == "" && strings.TrimSpace(part.Value) == "" {
				continue
			}
			segments = append(segments, keyStyle.Render(strings.TrimSpace(part.Label)+": ")+valueStyle.Render(strings.TrimSpace(part.Value)))
		}
		if trimmedHint := strings.TrimSpace(hint); trimmedHint != "" {
			segments = append(segments, hintStyle.Render(trimmedHint))
		}
		return barStyle.Render(strings.Join(segments, "  "))
	}
}
