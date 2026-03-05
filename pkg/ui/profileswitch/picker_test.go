package profileswitch_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	overlaywidget "github.com/go-go-golems/pinocchio/pkg/tui/widgets/overlay"
	"github.com/go-go-golems/pinocchio/pkg/ui/profileswitch"
)

func makeTestItems() []profileswitch.ProfileListItem {
	return []profileswitch.ProfileListItem{
		{ProfileSlug: gepprofiles.ProfileSlug("alpha"), DisplayName: "Alpha Model"},
		{ProfileSlug: gepprofiles.ProfileSlug("beta"), DisplayName: "Beta Model"},
		{ProfileSlug: gepprofiles.ProfileSlug("gamma"), DisplayName: "Gamma Model"},
		{ProfileSlug: gepprofiles.ProfileSlug("delta"), DisplayName: "Delta Model"},
		{ProfileSlug: gepprofiles.ProfileSlug("epsilon"), DisplayName: "Epsilon Model"},
	}
}

func TestPickerShowsItems(t *testing.T) {
	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "beta", &selected)
	m.SetSize(60, 20)

	v := m.View()
	if !strings.Contains(v, "alpha") {
		t.Fatal("view should contain 'alpha'")
	}
	if !strings.Contains(v, "beta") {
		t.Fatal("view should contain 'beta'")
	}
}

func TestPickerCurrentMarker(t *testing.T) {
	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "beta", &selected)
	m.SetSize(60, 20)

	v := m.View()
	if !strings.Contains(v, "* beta") {
		t.Fatalf("current profile should be marked with *, got:\n%s", v)
	}
}

func TestPickerNavigation(t *testing.T) {
	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "alpha", &selected)
	m.SetSize(60, 20)

	// Move down.
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(*profileswitch.PickerModel)

	// Select with enter.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if selected != "beta" {
		t.Fatalf("expected 'beta' selected, got %q", selected)
	}

	// Should produce a CloseOverlayMsg.
	if cmd == nil {
		t.Fatal("enter should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(overlaywidget.CloseOverlayMsg); !ok {
		t.Fatalf("expected CloseOverlayMsg, got %T", msg)
	}
}

func TestPickerFilter(t *testing.T) {
	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "alpha", &selected)
	m.SetSize(60, 20)

	// Type "gam" to filter.
	for _, ch := range "gam" {
		model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = model.(*profileswitch.PickerModel)
	}

	v := m.View()
	if !strings.Contains(v, "gamma") {
		t.Fatal("filter should show gamma")
	}
	if strings.Contains(v, "alpha") {
		t.Fatal("filter should hide non-matching items")
	}

	// Enter should select the filtered item.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if selected != "gamma" {
		t.Fatalf("expected 'gamma' selected after filter, got %q", selected)
	}
}

func TestPickerFilterBackspace(t *testing.T) {
	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "alpha", &selected)
	m.SetSize(60, 20)

	// Type "xyz" (no matches).
	for _, ch := range "xyz" {
		model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = model.(*profileswitch.PickerModel)
	}

	// Backspace to clear.
	for i := 0; i < 3; i++ {
		model, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = model.(*profileswitch.PickerModel)
	}

	v := m.View()
	if !strings.Contains(v, "alpha") {
		t.Fatal("clearing filter should restore all items")
	}
}

func TestPickerEmptyItems(t *testing.T) {
	var selected string
	m := profileswitch.NewPickerModel(nil, "", &selected)
	m.SetSize(60, 20)

	v := m.View()
	if !strings.Contains(v, "No profiles") {
		t.Fatal("empty items should show 'No profiles' message")
	}
}

func TestPickerHeightConstraint(t *testing.T) {
	// Create many items.
	items := make([]profileswitch.ProfileListItem, 30)
	for i := range items {
		slug := gepprofiles.ProfileSlug(strings.Repeat("x", 5))
		items[i] = profileswitch.ProfileListItem{ProfileSlug: slug}
	}

	var selected string
	m := profileswitch.NewPickerModel(items, "", &selected)
	m.SetSize(60, 10) // Very constrained height.

	v := m.View()
	lines := strings.Split(v, "\n")
	if len(lines) > 12 {
		t.Fatalf("view should respect height constraint, got %d lines", len(lines))
	}
}

func TestPickerDisplayNameInLabel(t *testing.T) {
	var selected string
	items := []profileswitch.ProfileListItem{
		{ProfileSlug: gepprofiles.ProfileSlug("test"), DisplayName: "My Test Profile"},
	}
	m := profileswitch.NewPickerModel(items, "", &selected)
	m.SetSize(60, 20)

	v := m.View()
	if !strings.Contains(v, "My Test Profile") {
		t.Fatalf("view should contain display name, got:\n%s", v)
	}
}
