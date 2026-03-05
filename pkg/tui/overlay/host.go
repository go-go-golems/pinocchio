package overlay

import (
	lipglossv2 "charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/go-go-golems/pinocchio/pkg/tui/widgets/formoverlay"
)

// Host wraps an inner tea.Model and composes canvas layer overlays on top.
// It follows the same pattern as bobatea's REPL overlay system (model.go:278-362)
// but lives in pinocchio for domain-specific overlay use (profile switching, etc.).
type Host struct {
	inner tea.Model

	formOverlay *formoverlay.FormOverlay

	width, height int
}

// Config configures the overlay host.
type Config struct {
	// FormOverlay is the optional form overlay to manage.
	// If nil, no form overlay is available.
	FormOverlay *formoverlay.FormOverlay
}

// NewHost creates an overlay host wrapping the given inner model.
func NewHost(inner tea.Model, cfg Config) Host {
	return Host{
		inner:       inner,
		formOverlay: cfg.FormOverlay,
	}
}

// Init delegates to the inner model's Init.
func (h Host) Init() tea.Cmd {
	return h.inner.Init()
}

// Update handles messages with overlay priority routing.
// When the form overlay is visible, it receives ALL messages (keys AND internal
// huh messages from Init/Update) so the form can function properly.
func (h Host) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		h.width, h.height = v.Width, v.Height
		// Keep the overlay informed of terminal size for dynamic sizing.
		if h.formOverlay != nil {
			h.formOverlay.SetTerminalSize(v.Width, v.Height)
		}
		// Pass window size to both overlay and inner model.
		if h.formOverlay != nil && h.formOverlay.IsVisible() {
			h.formOverlay.Update(msg)
		}
		innerModel, cmd := h.inner.Update(msg)
		h.inner = innerModel
		return h, cmd

	case OpenFormOverlayMsg:
		cmd := h.openFormOverlay()
		return h, cmd

	case tea.KeyMsg:
		// Form overlay has highest priority when visible.
		if handled, cmd := h.handleFormOverlayInput(v); handled {
			return h, cmd
		}

		// Default: forward to inner model.
		innerModel, cmd := h.inner.Update(msg)
		h.inner = innerModel
		return h, cmd

	default:
		// When form overlay is visible, route non-key messages to it.
		// This is critical: huh forms emit internal messages from Init()
		// that must be processed for the form to work (focus, cursor, etc.).
		if h.formOverlay != nil && h.formOverlay.IsVisible() {
			cmd := h.formOverlay.Update(msg)
			return h, cmd
		}

		innerModel, cmd := h.inner.Update(msg)
		h.inner = innerModel
		return h, cmd
	}
}

// View composites the inner model's view with overlay layers using lipgloss v2 canvas.
func (h Host) View() string {
	base := h.inner.View()

	if h.width <= 0 || h.height <= 0 {
		return base
	}

	formLayout, formOK := h.computeFormOverlayLayout()

	if !formOK {
		return base
	}

	layers := []*lipglossv2.Layer{
		lipglossv2.NewLayer(base).X(0).Y(0).Z(0).ID("host-base"),
		lipglossv2.NewLayer(formLayout.View).
			X(formLayout.X).Y(formLayout.Y).Z(28).
			ID("form-overlay"),
	}

	comp := lipglossv2.NewCompositor(layers...)
	canvas := lipglossv2.NewCanvas(h.width, h.height)
	canvas.Compose(comp)
	return canvas.Render()
}

// SetFormOverlay sets the form overlay at runtime.
// This allows registering overlays after host creation.
func (h *Host) SetFormOverlay(fo *formoverlay.FormOverlay) {
	h.formOverlay = fo
}

// FormOverlayVisible returns whether the form overlay is currently shown.
func (h Host) FormOverlayVisible() bool {
	return h.formOverlay != nil && h.formOverlay.IsVisible()
}

// Inner returns the wrapped inner model.
func (h Host) Inner() tea.Model {
	return h.inner
}
