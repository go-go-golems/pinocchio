package formoverlay

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Placement determines where the overlay appears on screen.
type Placement int

const (
	PlacementCenter Placement = iota
	PlacementTop
	PlacementTopRight
)

// Config configures a FormOverlay.
type Config struct {
	// Title shown in the overlay's title bar.
	Title string

	// MaxWidth and MaxHeight constrain the overlay's rendered size.
	// If zero, defaults are used (60 and 20 respectively).
	MaxWidth  int
	MaxHeight int

	// Placement determines the overlay's position on screen.
	Placement Placement

	// Factory creates a fresh huh.Form each time the overlay opens.
	// Using a factory avoids huh's state-lock problem (form is dead
	// after StateCompleted). Each Show() call gets a new form.
	Factory func() *huh.Form

	// OnSubmit is called when the form completes successfully.
	// The caller can extract values via form.Get() or bound pointers.
	OnSubmit func(form *huh.Form)

	// OnCancel is called when the overlay is dismissed (Esc/ctrl+c).
	OnCancel func()

	// DoubleEscToClose requires two consecutive Esc presses to close the
	// overlay. This allows huh Select/MultiSelect fields to use the first
	// Esc to clear their filter. When false (default), a single Esc closes.
	DoubleEscToClose bool

	// BorderStyle is the lipgloss style for the modal border.
	// If zero value, DefaultBorderStyle() is used.
	BorderStyle lipgloss.Style

	// TitleStyle is the lipgloss style for the title bar.
	// If zero value, DefaultTitleStyle() is used.
	TitleStyle lipgloss.Style
}
