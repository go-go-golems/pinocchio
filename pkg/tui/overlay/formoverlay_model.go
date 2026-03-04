package overlay

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleFormOverlayInput routes key messages to the form overlay.
// Returns (handled, cmd). When the overlay is visible, ALL keys go to it.
func (h *Host) handleFormOverlayInput(k tea.KeyMsg) (bool, tea.Cmd) {
	if h.formOverlay == nil {
		return false, nil
	}

	// While visible: route all keys to the overlay (modal behavior).
	if h.formOverlay.IsVisible() {
		wasVisible := true
		cmd := h.formOverlay.Update(k)

		// If the overlay just closed, emit completion/cancellation message.
		if wasVisible && !h.formOverlay.IsVisible() {
			// The FormOverlay's onSubmit/onCancel callbacks have already fired.
			// No additional action needed here.
			return true, cmd
		}

		return true, cmd
	}

	return false, nil
}

// openFormOverlay shows the form overlay.
func (h *Host) openFormOverlay() tea.Cmd {
	if h.formOverlay == nil {
		return nil
	}
	return h.formOverlay.Show()
}
