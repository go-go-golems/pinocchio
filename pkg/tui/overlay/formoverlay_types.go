// Package overlay provides a compositing overlay host for Bubble Tea models.
// It follows the same canvas layer patterns as bobatea's REPL overlay system,
// adding FormOverlay support for modal huh forms.
package overlay

import (
	"github.com/charmbracelet/huh"
)

// OpenFormOverlayMsg tells the overlay host to open the form overlay.
type OpenFormOverlayMsg struct{}

// FormOverlayCompletedMsg is sent when the form overlay completes successfully.
// The caller can extract values via form.Get() or bound pointers.
type FormOverlayCompletedMsg struct {
	Form *huh.Form
}

// FormOverlayCancelledMsg is sent when the form overlay is dismissed.
type FormOverlayCancelledMsg struct{}
