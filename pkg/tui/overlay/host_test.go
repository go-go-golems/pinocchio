package overlay_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/go-go-golems/pinocchio/pkg/tui/overlay"
	"github.com/go-go-golems/pinocchio/pkg/tui/widgets/formoverlay"
)

// mockModel is a minimal tea.Model for testing the overlay host.
type mockModel struct {
	lastMsg  tea.Msg
	viewText string
}

func (m mockModel) Init() tea.Cmd                           { return nil }
func (m mockModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { m.lastMsg = msg; return m, nil }
func (m mockModel) View() string                            { return m.viewText }

func makeTestFormOverlay() *formoverlay.FormOverlay {
	return formoverlay.New(formoverlay.Config{
		Title: "Test Form",
		Factory: func() *huh.Form {
			var val string
			return huh.NewForm(
				huh.NewGroup(
					huh.NewInput().Title("Name").Value(&val),
				),
			)
		},
		MaxWidth:  40,
		MaxHeight: 10,
		Placement: formoverlay.PlacementCenter,
	})
}

func TestHostDelegatesToInner(t *testing.T) {
	inner := mockModel{viewText: "hello world"}
	host := overlay.NewHost(inner, overlay.Config{})

	// Init should delegate to inner.
	cmd := host.Init()
	if cmd != nil {
		t.Fatal("init should return nil for mock model")
	}

	// View should return inner's view when no overlay is active.
	v := host.View()
	if v != "hello world" {
		t.Fatalf("expected inner view, got %q", v)
	}
}

func TestHostFormOverlayVisibility(t *testing.T) {
	inner := mockModel{viewText: "base"}
	fo := makeTestFormOverlay()
	host := overlay.NewHost(inner, overlay.Config{FormOverlay: fo})

	if host.FormOverlayVisible() {
		t.Fatal("form overlay should start hidden")
	}

	// Open via message.
	model, _ := host.Update(overlay.OpenFormOverlayMsg{})
	host = model.(overlay.Host)

	if !host.FormOverlayVisible() {
		t.Fatal("form overlay should be visible after OpenFormOverlayMsg")
	}
}

func TestHostKeyRoutingToOverlay(t *testing.T) {
	inner := &mockModel{viewText: "base"}
	fo := makeTestFormOverlay()
	host := overlay.NewHost(inner, overlay.Config{FormOverlay: fo})

	// Open overlay.
	model, _ := host.Update(overlay.OpenFormOverlayMsg{})
	host = model.(overlay.Host)

	// Send a key — should go to overlay, not inner.
	model, _ = host.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	host = model.(overlay.Host)

	// Overlay should still be visible (regular key doesn't close it).
	if !host.FormOverlayVisible() {
		t.Fatal("regular key should not close overlay")
	}
}

func TestHostEscClosesOverlay(t *testing.T) {
	inner := mockModel{viewText: "base"}
	fo := makeTestFormOverlay()
	host := overlay.NewHost(inner, overlay.Config{FormOverlay: fo})

	// Open overlay.
	model, _ := host.Update(overlay.OpenFormOverlayMsg{})
	host = model.(overlay.Host)

	// Esc should close overlay.
	model, _ = host.Update(tea.KeyMsg{Type: tea.KeyEscape})
	host = model.(overlay.Host)

	if host.FormOverlayVisible() {
		t.Fatal("Esc should close overlay")
	}
}

func TestHostViewComposesLayers(t *testing.T) {
	inner := mockModel{viewText: "base content here"}
	fo := makeTestFormOverlay()
	host := overlay.NewHost(inner, overlay.Config{FormOverlay: fo})

	// Set terminal size so canvas works.
	model, _ := host.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	host = model.(overlay.Host)

	// Without overlay: should return base.
	v := host.View()
	if !strings.Contains(v, "base content") {
		t.Fatal("view should contain base content when overlay is hidden")
	}

	// Open overlay.
	model, _ = host.Update(overlay.OpenFormOverlayMsg{})
	host = model.(overlay.Host)

	// With overlay: should contain both base layer and overlay content.
	v = host.View()
	if v == "" {
		t.Fatal("view should not be empty with overlay open")
	}
	// The canvas render should produce non-empty output.
	if len(v) < 10 {
		t.Fatal("view with overlay should be substantial")
	}
}

func TestHostKeyRoutingReturnsToInnerAfterClose(t *testing.T) {
	inner := mockModel{viewText: "base"}
	fo := makeTestFormOverlay()
	host := overlay.NewHost(inner, overlay.Config{FormOverlay: fo})

	// Open, then close.
	model, _ := host.Update(overlay.OpenFormOverlayMsg{})
	host = model.(overlay.Host)
	model, _ = host.Update(tea.KeyMsg{Type: tea.KeyEscape})
	host = model.(overlay.Host)

	// Now keys should go to inner model.
	if host.FormOverlayVisible() {
		t.Fatal("overlay should be closed")
	}

	// View should be base content again.
	v := host.View()
	if v != "base" {
		t.Fatalf("expected base view after close, got %q", v)
	}
}

func TestHostWindowSizeTracked(t *testing.T) {
	inner := mockModel{viewText: "base"}
	host := overlay.NewHost(inner, overlay.Config{})

	model, _ := host.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	host = model.(overlay.Host)

	// View should work (no panic with tracked dimensions).
	v := host.View()
	if v != "base" {
		t.Fatalf("expected base view, got %q", v)
	}
}

func TestHostSetFormOverlay(t *testing.T) {
	inner := mockModel{viewText: "base"}
	host := overlay.NewHost(inner, overlay.Config{})

	if host.FormOverlayVisible() {
		t.Fatal("no overlay should be visible initially")
	}

	// Set overlay at runtime.
	fo := makeTestFormOverlay()
	host.SetFormOverlay(fo)

	// Open it.
	model, _ := host.Update(overlay.OpenFormOverlayMsg{})
	host = model.(overlay.Host)

	if !host.FormOverlayVisible() {
		t.Fatal("overlay should be visible after runtime registration + open")
	}
}

func TestHostNoFormOverlay(t *testing.T) {
	inner := mockModel{viewText: "base"}
	host := overlay.NewHost(inner, overlay.Config{})

	// OpenFormOverlayMsg with no overlay should be a no-op.
	model, cmd := host.Update(overlay.OpenFormOverlayMsg{})
	host = model.(overlay.Host)

	if cmd != nil {
		t.Fatal("open with no overlay should return nil cmd")
	}
	if host.FormOverlayVisible() {
		t.Fatal("should not be visible without an overlay configured")
	}
}

func TestHostOnSubmitCallback(t *testing.T) {
	submitted := false
	inner := mockModel{viewText: "base"}
	fo := formoverlay.New(formoverlay.Config{
		Title: "Test",
		Factory: func() *huh.Form {
			var val string
			return huh.NewForm(
				huh.NewGroup(
					huh.NewInput().Title("Name").Value(&val),
				),
			)
		},
		OnSubmit: func(form *huh.Form) {
			submitted = true
		},
		MaxWidth:  40,
		MaxHeight: 10,
	})
	host := overlay.NewHost(inner, overlay.Config{FormOverlay: fo})

	// Open overlay.
	model, _ := host.Update(overlay.OpenFormOverlayMsg{})
	host = model.(overlay.Host)

	if !host.FormOverlayVisible() {
		t.Fatal("overlay should be visible")
	}

	// The onSubmit callback is tested at the FormOverlay level.
	// Here we just verify the host wiring doesn't interfere.
	_ = submitted
}
