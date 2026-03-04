// Example demonstrating FormOverlay as a modal dialog in a Bubble Tea app.
// Press 's' to open a select overlay, 'c' for confirm, 'i' for text input.
package main

import (
	"fmt"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/go-go-golems/pinocchio/pkg/tui/widgets/formoverlay"
)

type model struct {
	overlay  *formoverlay.FormOverlay
	messages []string
	width    int
	height   int
}

func newModel() model {
	return model{
		messages: []string{"Press s=select, c=confirm, i=input, q=quit"},
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Route to overlay first when visible.
	if m.overlay != nil && m.overlay.IsVisible() {
		cmd := m.overlay.Update(msg)
		if !m.overlay.IsVisible() {
			m.overlay = nil
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "s":
			m.overlay = formoverlay.NewSelect(
				"Pick a Color",
				[]huh.Option[string]{
					huh.NewOption("Red", "red"),
					huh.NewOption("Green", "green"),
					huh.NewOption("Blue", "blue"),
				},
				func(value string) {
					m.messages = append(m.messages, fmt.Sprintf("Selected: %s", value))
				},
			)
			return m, m.overlay.Show()

		case "c":
			m.overlay = formoverlay.NewConfirm(
				"Delete everything?",
				func(confirmed bool) {
					m.messages = append(m.messages, fmt.Sprintf("Confirmed: %v", confirmed))
				},
			)
			return m, m.overlay.Show()

		case "i":
			m.overlay = formoverlay.NewInput(
				"Enter your name",
				func(value string) {
					m.messages = append(m.messages, fmt.Sprintf("Input: %q", value))
				},
			)
			return m, m.overlay.Show()
		}
	}

	return m, nil
}

func (m model) View() string {
	var sb strings.Builder

	sb.WriteString("=== FormOverlay Example ===\n\n")

	// Show last 10 messages.
	start := 0
	if len(m.messages) > 10 {
		start = len(m.messages) - 10
	}
	for _, msg := range m.messages[start:] {
		sb.WriteString("  " + msg + "\n")
	}

	if m.overlay != nil && m.overlay.IsVisible() {
		sb.WriteString("\n")
		sb.WriteString(m.overlay.View())
	}

	return sb.String()
}

func main() {
	if _, err := tea.NewProgram(newModel(), tea.WithAltScreen()).Run(); err != nil {
		log.Fatal(err)
	}
}
