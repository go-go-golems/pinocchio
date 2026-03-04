package formoverlay_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/go-go-golems/pinocchio/pkg/tui/widgets/formoverlay"
	uhoh "github.com/go-go-golems/uhoh/pkg"
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

// --- uhoh integration tests ---

func TestUhohFormEmbedding(t *testing.T) {
	yamlSrc := []byte(`
name: Test Form
groups:
  - name: Demo
    fields:
      - type: input
        key: name
        title: Your Name
        value: Claude
      - type: confirm
        key: ok
        title: Continue?
        value: true
`)
	var submitted bool
	var formValues map[string]interface{}

	factory := func() *huh.Form {
		f, vals, err := uhoh.BuildBubbleTeaModelFromYAML(yamlSrc)
		if err != nil {
			t.Fatalf("BuildBubbleTeaModelFromYAML failed: %v", err)
		}
		formValues = vals
		_ = formValues // values available for extraction after submit
		return f
	}

	o := formoverlay.New(formoverlay.Config{
		Title:   "uhoh Form",
		Factory: factory,
		OnSubmit: func(form *huh.Form) {
			submitted = true
		},
	})

	o.Show()

	if !o.IsVisible() {
		t.Fatal("overlay should be visible after Show with uhoh form")
	}

	v := o.View()
	if v == "" {
		t.Fatal("uhoh form overlay should render non-empty view")
	}
	if !strings.Contains(v, "uhoh Form") {
		t.Fatalf("expected title 'uhoh Form' in view, got:\n%s", v)
	}

	// Esc should close it.
	o.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if o.IsVisible() {
		t.Fatal("Esc should close uhoh form overlay")
	}

	// Reopen to verify factory creates fresh form.
	o.Show()
	if !o.IsVisible() {
		t.Fatal("second Show should reopen overlay with fresh form")
	}

	_ = submitted // not testing completion flow here, just embedding
}

func TestUhohMultiGroupFormEmbedding(t *testing.T) {
	yamlSrc := []byte(`
name: Multi-Group Form
groups:
  - name: Step 1
    fields:
      - type: input
        key: first_name
        title: First Name
  - name: Step 2
    fields:
      - type: input
        key: last_name
        title: Last Name
  - name: Step 3
    fields:
      - type: confirm
        key: agree
        title: Do you agree?
`)
	factory := func() *huh.Form {
		f, _, err := uhoh.BuildBubbleTeaModelFromYAML(yamlSrc)
		if err != nil {
			t.Fatalf("BuildBubbleTeaModelFromYAML failed: %v", err)
		}
		return f
	}

	o := formoverlay.New(formoverlay.Config{
		Title:   "Wizard",
		Factory: factory,
	})

	o.Show()
	_ = o.View()
}

func TestDoubleEscToClose(t *testing.T) {
	cancelled := false
	o := formoverlay.New(formoverlay.Config{
		Title:            "Test",
		Factory:          makeTestFactory(),
		DoubleEscToClose: true,
		OnCancel: func() {
			cancelled = true
		},
	})

	o.Show()

	// First Esc: should NOT close (passes through to form).
	o.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if !o.IsVisible() {
		t.Fatal("first Esc should not close with DoubleEscToClose")
	}
	if cancelled {
		t.Fatal("OnCancel should not fire on first Esc")
	}

	// Second Esc: should close.
	o.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if o.IsVisible() {
		t.Fatal("second Esc should close overlay")
	}
	if !cancelled {
		t.Fatal("OnCancel should fire on second Esc")
	}
}

func TestDoubleEscResetByOtherKey(t *testing.T) {
	o := formoverlay.New(formoverlay.Config{
		Title:            "Test",
		Factory:          makeTestFactory(),
		DoubleEscToClose: true,
	})

	o.Show()

	// First Esc.
	o.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if !o.IsVisible() {
		t.Fatal("first Esc should not close")
	}

	// Regular key resets the pending state.
	o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	// Esc again: should be treated as first Esc (not second).
	o.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if !o.IsVisible() {
		t.Fatal("Esc after key reset should not close (back to first Esc)")
	}

	// Now second Esc closes.
	o.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if o.IsVisible() {
		t.Fatal("second consecutive Esc should close")
	}
}
