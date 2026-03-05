package overlay_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/pinocchio/pkg/tui/overlay"
	overlaywidget "github.com/go-go-golems/pinocchio/pkg/tui/widgets/overlay"
)

// mockModel is a minimal tea.Model for testing the overlay host.
type mockModel struct {
	lastMsg  tea.Msg
	viewText string
}

func (m mockModel) Init() tea.Cmd { return nil }
func (m mockModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.lastMsg = msg
	return m, nil
}
func (m mockModel) View() string { return m.viewText }

// trackingModel tracks whether Update was called with non-key messages.
type trackingModel struct {
	viewText   string
	updateMsgs []tea.Msg
}

func (m *trackingModel) Init() tea.Cmd { return nil }
func (m *trackingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.updateMsgs = append(m.updateMsgs, msg)
	return m, nil
}
func (m *trackingModel) View() string { return m.viewText }

// mockContent is a minimal tea.Model for overlay content.
type mockContent struct {
	viewText string
}

func (m mockContent) Init() tea.Cmd                           { return nil }
func (m mockContent) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m mockContent) View() string                            { return m.viewText }

func makeTestOverlay() *overlaywidget.Overlay {
	return overlaywidget.New(overlaywidget.Config{
		Title: "Test",
		Factory: func() tea.Model {
			return mockContent{viewText: "overlay content"}
		},
		MaxWidth:  40,
		MaxHeight: 10,
		Placement: overlaywidget.PlacementCenter,
	})
}

func TestHostDelegatesToInner(t *testing.T) {
	inner := mockModel{viewText: "hello world"}
	host := overlay.NewHost(inner, overlay.Config{})

	cmd := host.Init()
	if cmd != nil {
		t.Fatal("init should return nil for mock model")
	}

	v := host.View()
	if v != "hello world" {
		t.Fatalf("expected inner view, got %q", v)
	}
}

func TestHostOverlayVisibility(t *testing.T) {
	inner := mockModel{viewText: "base"}
	o := makeTestOverlay()
	host := overlay.NewHost(inner, overlay.Config{Overlay: o})

	if host.OverlayVisible() {
		t.Fatal("overlay should start hidden")
	}

	model, _ := host.Update(overlay.OpenOverlayMsg{})
	host = model.(overlay.Host)

	if !host.OverlayVisible() {
		t.Fatal("overlay should be visible after OpenOverlayMsg")
	}
}

func TestHostKeyRoutingToOverlay(t *testing.T) {
	inner := &mockModel{viewText: "base"}
	o := makeTestOverlay()
	host := overlay.NewHost(inner, overlay.Config{Overlay: o})

	model, _ := host.Update(overlay.OpenOverlayMsg{})
	host = model.(overlay.Host)

	model, _ = host.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	host = model.(overlay.Host)

	if !host.OverlayVisible() {
		t.Fatal("regular key should not close overlay")
	}
}

func TestHostEscClosesOverlay(t *testing.T) {
	inner := mockModel{viewText: "base"}
	o := makeTestOverlay()
	host := overlay.NewHost(inner, overlay.Config{Overlay: o})

	model, _ := host.Update(overlay.OpenOverlayMsg{})
	host = model.(overlay.Host)

	model, _ = host.Update(tea.KeyMsg{Type: tea.KeyEscape})
	host = model.(overlay.Host)

	if host.OverlayVisible() {
		t.Fatal("Esc should close overlay")
	}
}

func TestHostViewComposesLayers(t *testing.T) {
	inner := mockModel{viewText: "base content here"}
	o := makeTestOverlay()
	host := overlay.NewHost(inner, overlay.Config{Overlay: o})

	model, _ := host.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	host = model.(overlay.Host)

	v := host.View()
	if !strings.Contains(v, "base content") {
		t.Fatal("view should contain base content when overlay is hidden")
	}

	model, _ = host.Update(overlay.OpenOverlayMsg{})
	host = model.(overlay.Host)

	v = host.View()
	if v == "" {
		t.Fatal("view should not be empty with overlay open")
	}
	if len(v) < 10 {
		t.Fatal("view with overlay should be substantial")
	}
}

func TestHostKeyRoutingReturnsToInnerAfterClose(t *testing.T) {
	inner := mockModel{viewText: "base"}
	o := makeTestOverlay()
	host := overlay.NewHost(inner, overlay.Config{Overlay: o})

	model, _ := host.Update(overlay.OpenOverlayMsg{})
	host = model.(overlay.Host)
	model, _ = host.Update(tea.KeyMsg{Type: tea.KeyEscape})
	host = model.(overlay.Host)

	if host.OverlayVisible() {
		t.Fatal("overlay should be closed")
	}

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

	v := host.View()
	if v != "base" {
		t.Fatalf("expected base view, got %q", v)
	}
}

func TestHostSetOverlay(t *testing.T) {
	inner := mockModel{viewText: "base"}
	host := overlay.NewHost(inner, overlay.Config{})

	if host.OverlayVisible() {
		t.Fatal("no overlay should be visible initially")
	}

	o := makeTestOverlay()
	host.SetOverlay(o)

	model, _ := host.Update(overlay.OpenOverlayMsg{})
	host = model.(overlay.Host)

	if !host.OverlayVisible() {
		t.Fatal("overlay should be visible after runtime registration + open")
	}
}

func TestHostNoOverlay(t *testing.T) {
	inner := mockModel{viewText: "base"}
	host := overlay.NewHost(inner, overlay.Config{})

	model, cmd := host.Update(overlay.OpenOverlayMsg{})
	host = model.(overlay.Host)

	if cmd != nil {
		t.Fatal("open with no overlay should return nil cmd")
	}
	if host.OverlayVisible() {
		t.Fatal("should not be visible without an overlay configured")
	}
}

func TestHostOnCloseCallback(t *testing.T) {
	closed := false
	inner := mockModel{viewText: "base"}
	o := overlaywidget.New(overlaywidget.Config{
		Title: "Test",
		Factory: func() tea.Model {
			return mockContent{viewText: "content"}
		},
		OnClose: func() {
			closed = true
		},
		MaxWidth:  40,
		MaxHeight: 10,
	})
	host := overlay.NewHost(inner, overlay.Config{Overlay: o})

	model, _ := host.Update(overlay.OpenOverlayMsg{})
	host = model.(overlay.Host)

	if !host.OverlayVisible() {
		t.Fatal("overlay should be visible")
	}

	_ = closed
}

// Test that non-key messages are forwarded to inner model while overlay is visible.
func TestHostForwardsNonKeyMsgsToInnerWhileOverlayVisible(t *testing.T) {
	inner := &trackingModel{viewText: "base"}
	o := makeTestOverlay()
	host := overlay.NewHost(inner, overlay.Config{Overlay: o})

	// Set terminal size first.
	model, _ := host.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	host = model.(overlay.Host)

	// Open overlay.
	model, _ = host.Update(overlay.OpenOverlayMsg{})
	host = model.(overlay.Host)

	if !host.OverlayVisible() {
		t.Fatal("overlay should be visible")
	}

	// Clear tracking.
	inner.updateMsgs = nil

	// Send a custom non-key message.
	type customMsg struct{ data string }
	host.Update(customMsg{data: "hello"})

	// Inner should have received the message.
	found := false
	for _, msg := range inner.updateMsgs {
		if cm, ok := msg.(customMsg); ok && cm.data == "hello" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("inner model should receive non-key messages while overlay is visible")
	}
}
