package formoverlay_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/go-go-golems/pinocchio/pkg/tui/widgets/formoverlay"
)

func makeTestFactory() func() *huh.Form {
	return func() *huh.Form {
		var val string
		return huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Name").Value(&val),
			),
		)
	}
}

func TestShowHideLifecycle(t *testing.T) {
	o := formoverlay.New(formoverlay.Config{
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

func TestToggle(t *testing.T) {
	o := formoverlay.New(formoverlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	o.Toggle()
	if !o.IsVisible() {
		t.Fatal("first Toggle should show")
	}

	o.Toggle()
	if o.IsVisible() {
		t.Fatal("second Toggle should hide")
	}
}

func TestEscClosesOverlay(t *testing.T) {
	cancelled := false
	o := formoverlay.New(formoverlay.Config{
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
	o := formoverlay.New(formoverlay.Config{
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
	o := formoverlay.New(formoverlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	o.Show()

	// Send a regular key — overlay should stay visible and form gets it.
	o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if !o.IsVisible() {
		t.Fatal("regular keys should not close overlay")
	}
}

func TestFactoryCreatesFreshForm(t *testing.T) {
	callCount := 0
	factory := func() *huh.Form {
		callCount++
		return makeTestFactory()()
	}

	o := formoverlay.New(formoverlay.Config{
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
	o := formoverlay.New(formoverlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	if v := o.View(); v != "" {
		t.Fatalf("hidden overlay should return empty view, got %q", v)
	}
}

func TestViewWhenVisible(t *testing.T) {
	o := formoverlay.New(formoverlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	o.Show()
	v := o.View()
	if v == "" {
		t.Fatal("visible overlay should return non-empty view")
	}
}

func TestComputeLayoutHidden(t *testing.T) {
	o := formoverlay.New(formoverlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	_, ok := o.ComputeLayout(120, 40)
	if ok {
		t.Fatal("hidden overlay should return ok=false")
	}
}

func TestComputeLayoutCenter(t *testing.T) {
	o := formoverlay.New(formoverlay.Config{
		Title:     "Test",
		Factory:   makeTestFactory(),
		Placement: formoverlay.PlacementCenter,
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
	o := formoverlay.New(formoverlay.Config{
		Title:     "Test",
		Factory:   makeTestFactory(),
		Placement: formoverlay.PlacementTop,
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
	o := formoverlay.New(formoverlay.Config{
		Title:   "Test",
		Factory: makeTestFactory(),
	})

	cmd := o.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd != nil {
		t.Fatal("updating hidden overlay should return nil cmd")
	}
}

// --- Multi-group (wizard) tests ---

func makeMultiGroupFactory() func() *huh.Form {
	return func() *huh.Form {
		var name, email, bio string
		return huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Name").Value(&name),
			),
			huh.NewGroup(
				huh.NewInput().Title("Email").Value(&email),
			),
			huh.NewGroup(
				huh.NewText().Title("Bio").Value(&bio),
			),
		)
	}
}

func TestMultiGroupTitleShowsStepProgress(t *testing.T) {
	o := formoverlay.New(formoverlay.Config{
		Title:   "Wizard",
		Factory: makeMultiGroupFactory(),
	})

	o.Show()
	v := o.View()
	if !strings.Contains(v, "Step 1 of 3") {
		t.Fatalf("expected title to contain 'Step 1 of 3', got:\n%s", v)
	}
}

func TestSingleGroupTitleNoStepProgress(t *testing.T) {
	o := formoverlay.New(formoverlay.Config{
		Title:   "Simple",
		Factory: makeTestFactory(),
	})

	o.Show()
	v := o.View()
	if strings.Contains(v, "Step") {
		t.Fatalf("single-group form should not show step progress, got:\n%s", v)
	}
}

func TestMultiGroupFormStaysVisibleAcrossGroups(t *testing.T) {
	o := formoverlay.New(formoverlay.Config{
		Title:   "Wizard",
		Factory: makeMultiGroupFactory(),
	})

	o.Show()

	// Simulate sending regular keys — overlay should stay visible.
	for i := 0; i < 5; i++ {
		o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	}

	if !o.IsVisible() {
		t.Fatal("overlay should remain visible during multi-group form")
	}
}

func TestMultiGroupEscStillCloses(t *testing.T) {
	cancelled := false
	o := formoverlay.New(formoverlay.Config{
		Title:   "Wizard",
		Factory: makeMultiGroupFactory(),
		OnCancel: func() {
			cancelled = true
		},
	})

	o.Show()
	o.Update(tea.KeyMsg{Type: tea.KeyEscape})

	if o.IsVisible() {
		t.Fatal("Esc should close multi-group overlay")
	}
	if !cancelled {
		t.Fatal("OnCancel should fire on Esc in multi-group form")
	}
}
