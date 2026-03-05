// Package overlay provides a modal overlay widget that wraps any tea.Model
// for use as a floating dialog in Bubble Tea applications. It handles
// visibility lifecycle, key interception (esc/ctrl+c), dimension constraints,
// and canvas layer positioning.
package overlay

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CloseOverlayMsg signals that the content model wants to close the overlay.
// Content models should return this as a tea.Msg to dismiss themselves.
type CloseOverlayMsg struct{}

// Overlay wraps a tea.Model for use as a modal overlay.
type Overlay struct {
	content tea.Model
	factory func() tea.Model

	visible bool
	title   string

	placement Placement
	maxWidth  int
	maxHeight int

	// Terminal dimensions, set by SetTerminalSize.
	termWidth, termHeight int

	onClose  func()
	onCancel func()

	borderStyle lipgloss.Style
	titleStyle  lipgloss.Style
}

// New creates an Overlay from the given Config.
func New(cfg Config) *Overlay {
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

	return &Overlay{
		factory:     cfg.Factory,
		title:       cfg.Title,
		maxWidth:    maxW,
		maxHeight:   maxH,
		placement:   cfg.Placement,
		onClose:     cfg.OnClose,
		onCancel:    cfg.OnCancel,
		borderStyle: border,
		titleStyle:  title,
	}
}

// SetTerminalSize updates the known terminal dimensions.
func (o *Overlay) SetTerminalSize(w, h int) {
	o.termWidth = w
	o.termHeight = h
}

// Show creates a fresh content model from the factory and makes the overlay visible.
// Returns an Init command for the new content model.
func (o *Overlay) Show() tea.Cmd {
	if o.factory == nil {
		return nil
	}
	o.content = o.factory()
	o.visible = true
	return o.content.Init()
}

// Hide closes the overlay without triggering callbacks.
func (o *Overlay) Hide() {
	o.visible = false
	o.content = nil
}

// IsVisible returns whether the overlay is currently shown.
func (o *Overlay) IsVisible() bool {
	return o.visible
}

// Update routes messages to the content model with key interception.
// Esc and ctrl+c are intercepted to close the overlay (calling onCancel).
// CloseOverlayMsg from the content model triggers onClose and hides the overlay.
func (o *Overlay) Update(msg tea.Msg) tea.Cmd {
	if !o.visible || o.content == nil {
		return nil
	}

	// Intercept close message from content.
	if _, ok := msg.(CloseOverlayMsg); ok {
		if o.onClose != nil {
			o.onClose()
		}
		o.Hide()
		return nil
	}

	// Intercept esc/ctrl+c BEFORE content sees them.
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "ctrl+c", "esc":
			o.Hide()
			if o.onCancel != nil {
				o.onCancel()
			}
			return nil
		}
	}

	// Delegate to content model.
	model, cmd := o.content.Update(msg)
	o.content = model
	return cmd
}

// View renders the content inside a modal frame with border and title.
// Content is clipped before the border is applied so the border is never truncated.
func (o *Overlay) View() string {
	if !o.visible || o.content == nil {
		return ""
	}

	content := strings.TrimRight(o.content.View(), "\n")

	// Compute vertical frame cost (border + padding + title).
	vFrame := o.borderStyle.GetVerticalFrameSize()
	titleHeight := 0
	if o.title != "" {
		titleHeight = lipgloss.Height(o.titleStyle.Render(o.title)) + 1 // +1 for newline after title
	}

	// Clip content to fit within the effective max height, accounting for chrome.
	maxContentHeight := o.effectiveMaxHeight() - vFrame - titleHeight
	if maxContentHeight < 1 {
		maxContentHeight = 1
	}
	content = clipHeight(content, maxContentHeight)

	// Clip content width.
	maxContentWidth := o.ContentWidth()
	if maxContentWidth > 0 {
		content = clipWidth(content, maxContentWidth)
	}

	var frame strings.Builder
	if o.title != "" {
		frame.WriteString(o.titleStyle.Render(o.title))
		frame.WriteString("\n")
	}
	frame.WriteString(content)

	rendered := o.borderStyle.
		MaxWidth(o.effectiveMaxWidth()).
		Render(frame.String())

	return rendered
}

// OverlayLayout holds the computed position and rendered content for a canvas layer.
type OverlayLayout struct {
	X, Y int
	View string
}

// ComputeLayout calculates the overlay position for a canvas layer.
func (o *Overlay) ComputeLayout(termWidth, termHeight int) (OverlayLayout, bool) {
	if !o.visible || o.content == nil {
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
func (o *Overlay) effectiveMaxWidth() int {
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
func (o *Overlay) effectiveMaxHeight() int {
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

// ContentWidth returns the width available for content inside the border.
func (o *Overlay) ContentWidth() int {
	hFrame := o.borderStyle.GetHorizontalFrameSize()
	w := o.effectiveMaxWidth() - hFrame
	if w < 10 {
		w = 10
	}
	return w
}

// ContentHeight returns the height available for content inside the border.
func (o *Overlay) ContentHeight() int {
	vFrame := o.borderStyle.GetVerticalFrameSize()
	titleHeight := 0
	if o.title != "" {
		titleHeight = lipgloss.Height(o.titleStyle.Render(o.title)) + 1
	}
	h := o.effectiveMaxHeight() - vFrame - titleHeight
	if h < 1 {
		h = 1
	}
	return h
}

// clipHeight truncates a rendered string to at most maxLines lines.
func clipHeight(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

// clipWidth truncates each line to at most maxWidth visible characters.
func clipWidth(s string, maxWidth int) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > maxWidth {
			// Use lipgloss-aware truncation.
			lines[i] = lipgloss.NewStyle().MaxWidth(maxWidth).Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
