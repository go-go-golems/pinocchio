// Example demonstrating Overlay as a modal dialog in a Bubble Tea app.
// Press 's' to open a select overlay, 'q' to quit.
package main

import (
	"fmt"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	overlaywidget "github.com/go-go-golems/pinocchio/pkg/tui/widgets/overlay"
)

// selectModel is a simple list selection model for the overlay.
type selectModel struct {
	items   []string
	cursor  int
	onClose func(string)
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			if m.onClose != nil {
				m.onClose(m.items[m.cursor])
			}
			return m, func() tea.Msg { return overlaywidget.CloseOverlayMsg{} }
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	var sb strings.Builder
	for i, item := range m.items {
		if i == m.cursor {
			sb.WriteString(fmt.Sprintf("> %s\n", item))
		} else {
			sb.WriteString(fmt.Sprintf("  %s\n", item))
		}
	}
	return sb.String()
}

type model struct {
	overlay  *overlaywidget.Overlay
	messages []string
	width    int
	height   int
}

func newModel() model {
	return model{
		messages: []string{"Press s=select, q=quit"},
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
			var selected string
			m.overlay = overlaywidget.New(overlaywidget.Config{
				Title: "Pick a Color",
				Factory: func() tea.Model {
					return selectModel{
						items:  []string{"Red", "Green", "Blue"},
						cursor: 0,
						onClose: func(value string) {
							selected = value
						},
					}
				},
				OnClose: func() {
					m.messages = append(m.messages, fmt.Sprintf("Selected: %s", selected))
				},
			})
			return m, m.overlay.Show()
		}
	}

	return m, nil
}

func (m model) View() string {
	var sb strings.Builder

	sb.WriteString("=== Overlay Example ===\n\n")

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
