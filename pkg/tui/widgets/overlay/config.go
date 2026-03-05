package overlay

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Placement determines where the overlay appears on screen.
type Placement int

const (
	PlacementCenter Placement = iota
	PlacementTop
	PlacementTopRight
)

// Config configures an Overlay.
type Config struct {
	// Title shown in the overlay's title bar.
	Title string

	// MaxWidth and MaxHeight constrain the overlay's rendered size.
	// If zero, defaults are used (80 and 30 respectively).
	MaxWidth  int
	MaxHeight int

	// Placement determines the overlay's position on screen.
	Placement Placement

	// Factory creates a fresh content tea.Model each time the overlay opens.
	Factory func() tea.Model

	// OnClose is called when the content model sends CloseOverlayMsg
	// (successful completion).
	OnClose func()

	// OnCancel is called when the overlay is dismissed via Esc or ctrl+c.
	OnCancel func()

	// BorderStyle is the lipgloss style for the modal border.
	// If zero value, DefaultBorderStyle() is used.
	BorderStyle lipgloss.Style

	// TitleStyle is the lipgloss style for the title bar.
	// If zero value, DefaultTitleStyle() is used.
	TitleStyle lipgloss.Style
}
