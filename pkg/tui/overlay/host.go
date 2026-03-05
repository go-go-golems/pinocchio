package overlay

import (
	lipglossv2 "charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"

	overlaywidget "github.com/go-go-golems/pinocchio/pkg/tui/widgets/overlay"
)

// Host wraps an inner tea.Model and composes canvas layer overlays on top.
type Host struct {
	inner tea.Model

	overlay *overlaywidget.Overlay

	width, height int
}

// Config configures the overlay host.
type Config struct {
	// Overlay is the optional modal overlay to manage.
	Overlay *overlaywidget.Overlay
}

// NewHost creates an overlay host wrapping the given inner model.
func NewHost(inner tea.Model, cfg Config) Host {
	return Host{
		inner:   inner,
		overlay: cfg.Overlay,
	}
}

// Init delegates to the inner model's Init.
func (h Host) Init() tea.Cmd {
	return h.inner.Init()
}

// Update handles messages with overlay priority routing.
// When the overlay is visible, key messages go to the overlay.
// Non-key messages are forwarded to BOTH the overlay and the inner model.
func (h Host) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		h.width, h.height = v.Width, v.Height
		if h.overlay != nil {
			h.overlay.SetTerminalSize(v.Width, v.Height)
		}
		// Pass window size to both overlay and inner model.
		var cmds []tea.Cmd
		if h.overlay != nil && h.overlay.IsVisible() {
			cmds = append(cmds, h.overlay.Update(msg))
		}
		innerModel, innerCmd := h.inner.Update(msg)
		h.inner = innerModel
		cmds = append(cmds, innerCmd)
		return h, tea.Batch(cmds...)

	case OpenOverlayMsg:
		cmd := h.openOverlay()
		return h, cmd

	case tea.KeyMsg:
		// Overlay has highest priority when visible.
		if h.overlay != nil && h.overlay.IsVisible() {
			cmd := h.overlay.Update(v)
			return h, cmd
		}
		// Default: forward to inner model.
		innerModel, cmd := h.inner.Update(msg)
		h.inner = innerModel
		return h, cmd

	default:
		// Forward non-key messages to BOTH overlay and inner model.
		var cmds []tea.Cmd
		if h.overlay != nil && h.overlay.IsVisible() {
			cmds = append(cmds, h.overlay.Update(msg))
		}
		innerModel, innerCmd := h.inner.Update(msg)
		h.inner = innerModel
		cmds = append(cmds, innerCmd)
		return h, tea.Batch(cmds...)
	}
}

// View composites the inner model's view with overlay layers using lipgloss v2 canvas.
func (h Host) View() string {
	base := h.inner.View()

	if h.width <= 0 || h.height <= 0 {
		return base
	}

	if h.overlay == nil || !h.overlay.IsVisible() {
		return base
	}

	overlayLayout, ok := h.overlay.ComputeLayout(h.width, h.height)
	if !ok {
		return base
	}

	layers := []*lipglossv2.Layer{
		lipglossv2.NewLayer(base).X(0).Y(0).Z(0).ID("host-base"),
		lipglossv2.NewLayer(overlayLayout.View).
			X(overlayLayout.X).Y(overlayLayout.Y).Z(28).
			ID("overlay"),
	}

	comp := lipglossv2.NewCompositor(layers...)
	canvas := lipglossv2.NewCanvas(h.width, h.height)
	canvas.Compose(comp)
	return canvas.Render()
}

// SetOverlay sets the overlay at runtime.
func (h *Host) SetOverlay(o *overlaywidget.Overlay) {
	h.overlay = o
}

// OverlayVisible returns whether the overlay is currently shown.
func (h Host) OverlayVisible() bool {
	return h.overlay != nil && h.overlay.IsVisible()
}

// Inner returns the wrapped inner model.
func (h Host) Inner() tea.Model {
	return h.inner
}

// openOverlay shows the overlay.
func (h *Host) openOverlay() tea.Cmd {
	if h.overlay == nil {
		return nil
	}
	return h.overlay.Show()
}
