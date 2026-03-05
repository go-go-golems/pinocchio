package profileswitch

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	overlaywidget "github.com/go-go-golems/pinocchio/pkg/tui/widgets/overlay"
)

// PickerModel is a custom tea.Model for selecting a profile from a list.
// It handles keyboard navigation, filtering, and height-aware rendering.
type PickerModel struct {
	items    []ProfileListItem
	filtered []int // indices into items
	cursor   int   // position within filtered list
	filter   string

	selected    *string // bound value — written on submit
	currentSlug string

	width, height int // available content area

	// Styles
	selectedStyle   lipgloss.Style
	unselectedStyle lipgloss.Style
	currentStyle    lipgloss.Style
	filterStyle     lipgloss.Style
	helpStyle       lipgloss.Style
}

// NewPickerModel creates a PickerModel from profile items.
func NewPickerModel(items []ProfileListItem, currentSlug string, selected *string) *PickerModel {
	m := &PickerModel{
		items:       items,
		selected:    selected,
		currentSlug: currentSlug,
		selectedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Bold(true),
		unselectedStyle: lipgloss.NewStyle(),
		currentStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")),
		filterStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")),
		helpStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
	}
	m.rebuildFiltered()
	// Start cursor on the currently active profile.
	for i, idx := range m.filtered {
		if m.items[idx].ProfileSlug.String() == currentSlug {
			m.cursor = i
			break
		}
	}
	return m
}

// PickerFactory returns a factory function for use with overlay.Config.Factory.
func PickerFactory(mgr *Manager, selected *string) func() tea.Model {
	return func() tea.Model {
		items, err := mgr.ListProfiles(context.Background())
		if err != nil || len(items) == 0 {
			return NewPickerModel(nil, "", selected)
		}
		current := mgr.Current().ProfileSlug.String()
		return NewPickerModel(items, current, selected)
	}
}

func (m *PickerModel) Init() tea.Cmd { return nil }

func (m *PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "home":
			m.cursor = 0
		case "end":
			m.cursor = len(m.filtered) - 1
		case "enter":
			if len(m.filtered) == 0 {
				return m, nil
			}
			if m.selected != nil {
				idx := m.filtered[m.cursor]
				*m.selected = m.items[idx].ProfileSlug.String()
			}
			return m, func() tea.Msg { return overlaywidget.CloseOverlayMsg{} }
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.rebuildFiltered()
			}
		default:
			// Typing characters adds to filter.
			if len(msg.Runes) == 1 {
				m.filter += string(msg.Runes)
				m.rebuildFiltered()
			}
		}
	}
	return m, nil
}

func (m *PickerModel) View() string {
	if len(m.items) == 0 {
		return "No profiles available."
	}

	var sb strings.Builder

	// Filter indicator.
	if m.filter != "" {
		sb.WriteString(m.filterStyle.Render(fmt.Sprintf("filter: %s", m.filter)))
		sb.WriteString("\n\n")
	}

	// Visible range: scroll so cursor is always visible.
	visibleHeight := m.visibleItemCount()
	start := 0
	if m.cursor >= visibleHeight {
		start = m.cursor - visibleHeight + 1
	}
	end := start + visibleHeight
	if end > len(m.filtered) {
		end = len(m.filtered)
		start = max(0, end-visibleHeight)
	}

	for i := start; i < end; i++ {
		idx := m.filtered[i]
		it := m.items[idx]
		slug := it.ProfileSlug.String()

		// Build the line.
		marker := "  "
		if slug == m.currentSlug {
			marker = "* "
		}

		label := slug
		if name := strings.TrimSpace(it.DisplayName); name != "" && name != slug {
			label += " — " + name
		}

		line := marker + label

		// Highlight cursor row.
		if i == m.cursor {
			sb.WriteString(m.selectedStyle.Render(line))
		} else if slug == m.currentSlug {
			sb.WriteString(m.currentStyle.Render(line))
		} else {
			sb.WriteString(m.unselectedStyle.Render(line))
		}

		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	// Scroll indicators.
	if start > 0 || end < len(m.filtered) {
		sb.WriteString("\n")
		if start > 0 && end < len(m.filtered) {
			sb.WriteString(m.helpStyle.Render(fmt.Sprintf("  ↑ %d more  ↓ %d more", start, len(m.filtered)-end)))
		} else if start > 0 {
			sb.WriteString(m.helpStyle.Render(fmt.Sprintf("  ↑ %d more", start)))
		} else {
			sb.WriteString(m.helpStyle.Render(fmt.Sprintf("  ↓ %d more", len(m.filtered)-end)))
		}
	}

	// Help line.
	sb.WriteString("\n")
	sb.WriteString(m.helpStyle.Render("  ↑/↓ navigate  enter select  esc cancel  type to filter"))

	return sb.String()
}

// visibleItemCount returns how many items can fit in the available height.
func (m *PickerModel) visibleItemCount() int {
	available := m.height
	if available <= 0 {
		available = 15 // fallback
	}
	// Reserve lines for: help (1) + possible filter (2) + possible scroll indicator (1).
	reserved := 2
	if m.filter != "" {
		reserved += 2
	}
	count := available - reserved
	if count < 3 {
		count = 3
	}
	if count > len(m.filtered) {
		count = len(m.filtered)
	}
	return count
}

// SetSize sets the available content area dimensions.
func (m *PickerModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// rebuildFiltered rebuilds the filtered index list from the current filter string.
func (m *PickerModel) rebuildFiltered() {
	if m.filter == "" {
		m.filtered = make([]int, len(m.items))
		for i := range m.items {
			m.filtered[i] = i
		}
	} else {
		m.filtered = m.filtered[:0]
		lower := strings.ToLower(m.filter)
		for i, it := range m.items {
			slug := strings.ToLower(it.ProfileSlug.String())
			name := strings.ToLower(it.DisplayName)
			if strings.Contains(slug, lower) || strings.Contains(name, lower) {
				m.filtered = append(m.filtered, i)
			}
		}
	}
	// Clamp cursor.
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}
