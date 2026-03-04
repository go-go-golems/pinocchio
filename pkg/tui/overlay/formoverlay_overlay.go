package overlay

import (
	"github.com/go-go-golems/pinocchio/pkg/tui/widgets/formoverlay"
)

// computeFormOverlayLayout returns the form overlay's position and rendered view.
// Returns the layout and true if the overlay is visible, zero and false otherwise.
func (h *Host) computeFormOverlayLayout() (formoverlay.OverlayLayout, bool) {
	if h.formOverlay == nil {
		return formoverlay.OverlayLayout{}, false
	}
	return h.formOverlay.ComputeLayout(h.width, h.height)
}
