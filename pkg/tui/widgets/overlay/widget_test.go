package overlay_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/pinocchio/pkg/tui/widgets/overlay"
)

// mockContent is a minimal tea.Model for testing overlays.
type mockContent struct {
	viewText string
}

func (m mockContent) Init() tea.Cmd                           { return nil }
func (m mockContent) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m mockContent) View() string                            { return m.viewText }

func makeTestFactory() func() tea.Model {
	return func() tea.Model {
		return mockContent{viewText: "test content"}
	}
}

func TestShowHideLifecycle(t *testing.T) {
	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	if o.IsVisible() {
		t.Fatal("overlay should start hidden")
	}

	o.Show()
	if !o.IsVisible() {
		t.Fatal("overlay should be visible after Show")
	}

	o.Hide()
	if o.IsVisible() {
		t.Fatal("overlay should be hidden after Hide")
	}
}

func TestEscClosesOverlay(t *testing.T) {
	cancelled := false
	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
		OnCancel: func() {
			cancelled = true
		},
	})

	o.Show()
	o.Update(tea.KeyMsg{Type: tea.KeyEscape})

	if o.IsVisible() {
		t.Fatal("Esc should close overlay")
	}
	if !cancelled {
		t.Fatal("OnCancel should be called on Esc")
	}
}

func TestCtrlCClosesOverlay(t *testing.T) {
	cancelled := false
	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
		OnCancel: func() {
			cancelled = true
		},
	})

	o.Show()
	o.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if o.IsVisible() {
		t.Fatal("ctrl+c should close overlay")
	}
	if !cancelled {
		t.Fatal("OnCancel should be called on ctrl+c")
	}
}

func TestRegularKeysPassThrough(t *testing.T) {
	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	o.Show()
	o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if !o.IsVisible() {
		t.Fatal("regular keys should not close overlay")
	}
}

func TestCloseOverlayMsg(t *testing.T) {
	closed := false
	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
		OnClose: func() {
			closed = true
		},
	})

	o.Show()
	o.Update(overlay.CloseOverlayMsg{})

	if o.IsVisible() {
		t.Fatal("CloseOverlayMsg should close overlay")
	}
	if !closed {
		t.Fatal("OnClose should be called on CloseOverlayMsg")
	}
}

func TestCloseOverlayMsgDoesNotCallOnCancel(t *testing.T) {
	cancelled := false
	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
		OnClose: func() {},
		OnCancel: func() {
			cancelled = true
		},
	})

	o.Show()
	o.Update(overlay.CloseOverlayMsg{})

	if cancelled {
		t.Fatal("CloseOverlayMsg should call OnClose, not OnCancel")
	}
}

func TestFactoryCreatesFreshContent(t *testing.T) {
	callCount := 0
	factory := func() tea.Model {
		callCount++
		return mockContent{viewText: "fresh"}
	}

	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: factory,
	})

	o.Show()
	if callCount != 1 {
		t.Fatalf("expected 1 factory call, got %d", callCount)
	}

	o.Hide()
	o.Show()
	if callCount != 2 {
		t.Fatalf("expected 2 factory calls, got %d", callCount)
	}
}

func TestViewWhenHidden(t *testing.T) {
	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	if v := o.View(); v != "" {
		t.Fatalf("hidden overlay should return empty view, got %q", v)
	}
}

func TestViewWhenVisible(t *testing.T) {
	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	o.Show()
	v := o.View()
	if v == "" {
		t.Fatal("visible overlay should return non-empty view")
	}
	if !strings.Contains(v, "test content") {
		t.Fatalf("view should contain content, got:\n%s", v)
	}
	if !strings.Contains(v, "Test") {
		t.Fatalf("view should contain title, got:\n%s", v)
	}
}

func TestComputeLayoutHidden(t *testing.T) {
	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	_, ok := o.ComputeLayout(120, 40)
	if ok {
		t.Fatal("hidden overlay should return ok=false")
	}
}

func TestComputeLayoutCenter(t *testing.T) {
	o := overlay.New(overlay.Config{
		Title:     "Test",
		Factory:   makeTestFactory(),
		Placement: overlay.PlacementCenter,
		MaxWidth:  40,
		MaxHeight: 10,
	})

	o.Show()
	layout, ok := o.ComputeLayout(120, 40)
	if !ok {
		t.Fatal("visible overlay should return ok=true")
	}
	if layout.View == "" {
		t.Fatal("view should not be empty")
	}
	if layout.X <= 0 || layout.Y <= 0 {
		t.Fatalf("centered overlay should have positive x,y; got x=%d y=%d", layout.X, layout.Y)
	}
}

func TestComputeLayoutTop(t *testing.T) {
	o := overlay.New(overlay.Config{
		Title:     "Test",
		Factory:   makeTestFactory(),
		Placement: overlay.PlacementTop,
		MaxWidth:  40,
	})

	o.Show()
	layout, ok := o.ComputeLayout(120, 40)
	if !ok {
		t.Fatal("visible overlay should return ok=true")
	}
	if layout.Y != 2 {
		t.Fatalf("top placement should have y=2, got y=%d", layout.Y)
	}
}

func TestUpdateWhenHidden(t *testing.T) {
	o := overlay.New(overlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	cmd := o.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd != nil {
		t.Fatal("updating hidden overlay should return nil cmd")
	}
}

func TestContentHeightClipping(t *testing.T) {
	// Content that's taller than the overlay can hold.
	tallContent := strings.Repeat("line\n", 50)
	o := overlay.New(overlay.Config{
		Title: "Test",
		Factory: func() tea.Model {
			return mockContent{viewText: tallContent}
		},
		MaxHeight: 15,
	})

	o.SetTerminalSize(80, 25)
	o.Show()
	v := o.View()

	// The rendered view should fit within terminal constraints.
	viewHeight := strings.Count(v, "\n") + 1
	if viewHeight > 21 { // 25 - 4 margin
		t.Fatalf("view should be clipped to fit terminal, got %d lines", viewHeight)
	}

	// Border should not be clipped (check for closing border character).
	lines := strings.Split(v, "\n")
	lastLine := lines[len(lines)-1]
	if !strings.Contains(lastLine, "╰") && !strings.Contains(lastLine, "└") {
		t.Fatalf("bottom border should be intact, last line: %q", lastLine)
	}
}

func TestSetTerminalSize(t *testing.T) {
	o := overlay.New(overlay.Config{
		Title:    "Test",
		Factory:  makeTestFactory(),
		MaxWidth: 100,
	})

	o.SetTerminalSize(60, 20)

	// ContentWidth should be capped by terminal size.
	cw := o.ContentWidth()
	if cw >= 100 {
		t.Fatalf("content width should be capped by terminal, got %d", cw)
	}
}
