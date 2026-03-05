// Package formoverlay provides a modal overlay widget that wraps huh.Form
// for use as a floating dialog in Bubble Tea applications. It handles
// visibility lifecycle, key interception, dimension constraints, and
// canvas layer positioning.
package formoverlay

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// FormOverlay wraps a huh.Form for use as a modal overlay.
type FormOverlay struct {
	form    *huh.Form
	factory func() *huh.Form

	visible bool
	title   string

	placement Placement
	maxWidth  int
	maxHeight int

	// Terminal dimensions, set by SetTerminalSize.
	// Used to compute effective overlay size (capped to fit screen).
	termWidth, termHeight int

	onSubmit func(form *huh.Form)
	onCancel func()

	doubleEscToClose bool
	escPending       bool // true after the first Esc in double-Esc mode

	borderStyle lipgloss.Style
	titleStyle  lipgloss.Style
}

// New creates a FormOverlay from the given Config.
func New(cfg Config) *FormOverlay {
	maxW := cfg.MaxWidth
	if maxW == 0 {
		maxW = defaultMaxWidth
	}
	maxH := cfg.MaxHeight
	if maxH == 0 {
		maxH = defaultMaxHeight
	}
	border := cfg.BorderStyle
	if border.Value() == "" {
		border = DefaultBorderStyle()
	}
	title := cfg.TitleStyle
	if title.Value() == "" {
		title = DefaultTitleStyle()
	}

	return &FormOverlay{
		factory:          cfg.Factory,
		title:            cfg.Title,
		maxWidth:         maxW,
		maxHeight:        maxH,
		placement:        cfg.Placement,
		onSubmit:         cfg.OnSubmit,
		onCancel:         cfg.OnCancel,
		doubleEscToClose: cfg.DoubleEscToClose,
		borderStyle:      border,
		titleStyle:       title,
	}
}

// SetTerminalSize updates the known terminal dimensions.
// The overlay host should call this on every WindowSizeMsg so the overlay
// can size itself appropriately relative to the terminal.
func (o *FormOverlay) SetTerminalSize(w, h int) {
	o.termWidth = w
	o.termHeight = h

	// If visible, update the form's content width to match.
	if o.visible && o.form != nil {
		cw := o.contentWidth()
		if cw > 0 {
			o.form = o.form.WithWidth(cw)
		}
	}
}

// Show creates a fresh form from the factory and makes the overlay visible.
// Returns an Init command for the new form.
func (o *FormOverlay) Show() tea.Cmd {
	if o.factory == nil {
		return nil
	}
	o.form = o.factory()
	o.visible = true
	o.escPending = false

	// Set the content width so huh knows how wide to render fields.
	// Account for border padding so the form fits inside the modal.
	contentWidth := o.contentWidth()
	if contentWidth > 0 {
		o.form = o.form.WithWidth(contentWidth)
	}

	return o.form.Init()
}

// Hide closes the overlay without triggering callbacks.
func (o *FormOverlay) Hide() {
	o.visible = false
	o.form = nil
}

// Toggle opens or closes the overlay.
func (o *FormOverlay) Toggle() tea.Cmd {
	if o.visible {
		o.Hide()
		return nil
	}
	return o.Show()
}

// IsVisible returns whether the overlay is currently shown.
func (o *FormOverlay) IsVisible() bool {
	return o.visible
}

// Update routes messages to the inner form with key interception.
// Returns a Bubble Tea command. The caller should check IsVisible()
// after calling Update — it may have changed (form completed/cancelled).
func (o *FormOverlay) Update(msg tea.Msg) tea.Cmd {
	if !o.visible || o.form == nil {
		return nil
	}

	// Intercept keys BEFORE the form sees them.
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "ctrl+c":
			o.Hide()
			if o.onCancel != nil {
				o.onCancel()
			}
			return nil
		case "esc":
			if o.doubleEscToClose {
				if o.escPending {
					// Second Esc — close the overlay.
					o.escPending = false
					o.Hide()
					if o.onCancel != nil {
						o.onCancel()
					}
					return nil
				}
				// First Esc — pass through to form (for filter clear),
				// but mark as pending so the next Esc closes.
				o.escPending = true
				// Fall through to let the form handle this Esc.
			} else {
				o.Hide()
				if o.onCancel != nil {
					o.onCancel()
				}
				return nil
			}
		default:
			// Any non-Esc key resets the double-Esc state.
			o.escPending = false
		}
	}

	// Delegate to the inner form.
	model, cmd := o.form.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		o.form = f
	}

	// Check for form completion.
	if o.form != nil {
		switch o.form.State {
		case huh.StateNormal:
			// Form still active, nothing to do.
		case huh.StateCompleted:
			if o.onSubmit != nil {
				o.onSubmit(o.form)
			}
			o.Hide()
			return cmd
		case huh.StateAborted:
			if o.onCancel != nil {
				o.onCancel()
			}
			o.Hide()
			return cmd
		}
	}

	return cmd
}

// View renders the form inside a modal frame, constrained to max dimensions.
func (o *FormOverlay) View() string {
	if !o.visible || o.form == nil {
		return ""
	}

	content := strings.TrimSuffix(o.form.View(), "\n\n")

	var frame strings.Builder
	if o.title != "" {
		title := o.title
		// Show step progress for multi-group forms.
		// if o.form.GroupCount() > 1 {
		// title = fmt.Sprintf("%s (Step %d of %d)", o.title, o.form.GroupIndex()+1, o.form.GroupCount())
		// }
		frame.WriteString(o.titleStyle.Render(title))
		frame.WriteString("\n")
	}
	frame.WriteString(content)

	rendered := o.borderStyle.
		MaxWidth(o.effectiveMaxWidth()).
		MaxHeight(o.effectiveMaxHeight()).
		Render(frame.String())

	return rendered
}

// ComputeLayout calculates the overlay position for a canvas layer.
// Returns (x, y, view, ok). ok is false if the overlay is not visible.
// OverlayLayout holds the computed position and rendered content for a canvas layer.
type OverlayLayout struct {
	X, Y int
	View string
}

// ComputeLayout calculates the overlay position for a canvas layer.
// Returns the layout and true if the overlay is visible, or zero and false otherwise.
func (o *FormOverlay) ComputeLayout(termWidth, termHeight int) (OverlayLayout, bool) {
	if !o.visible || o.form == nil {
		return OverlayLayout{}, false
	}

	rendered := o.View()
	panelWidth := lipgloss.Width(rendered)
	panelHeight := lipgloss.Height(rendered)

	if panelWidth <= 0 || panelHeight <= 0 {
		return OverlayLayout{}, false
	}

	var lx, ly int
	switch o.placement {
	case PlacementCenter:
		lx = (termWidth - panelWidth) / 2
		ly = (termHeight - panelHeight) / 2
	case PlacementTop:
		lx = (termWidth - panelWidth) / 2
		ly = 2
	case PlacementTopRight:
		lx = termWidth - panelWidth - 2
		ly = 2
	}

	// Clamp to bounds.
	lx = max(0, min(lx, termWidth-panelWidth))
	ly = max(0, min(ly, termHeight-panelHeight))

	return OverlayLayout{X: lx, Y: ly, View: rendered}, true
}

// effectiveMaxWidth returns the overlay max width, capped to fit the terminal.
// Leaves a small margin (4 columns) so the overlay doesn't touch the edges.
func (o *FormOverlay) effectiveMaxWidth() int {
	w := o.maxWidth
	if o.termWidth > 0 {
		termCap := o.termWidth - 4
		if termCap < w {
			w = termCap
		}
	}
	if w < 20 {
		w = 20
	}
	return w
}

// effectiveMaxHeight returns the overlay max height, capped to fit the terminal.
// Leaves a margin (4 rows) so the overlay doesn't touch top/bottom.
func (o *FormOverlay) effectiveMaxHeight() int {
	h := o.maxHeight
	if o.termHeight > 0 {
		termCap := o.termHeight - 4
		if termCap < h {
			h = termCap
		}
	}
	if h < 10 {
		h = 10
	}
	return h
}

// contentWidth returns the width available for form content inside the border.
func (o *FormOverlay) contentWidth() int {
	hFrame := o.borderStyle.GetHorizontalFrameSize()
	w := o.effectiveMaxWidth() - hFrame
	if w < 10 {
		w = 10
	}
	return w
}
