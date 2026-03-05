package profileswitch_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	overlaywidget "github.com/go-go-golems/pinocchio/pkg/tui/widgets/overlay"
	"github.com/go-go-golems/pinocchio/pkg/ui/profileswitch"
	"github.com/stretchr/testify/require"
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
	t.Parallel()

	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "beta", &selected)
	m.SetSize(60, 20)

	v := m.View()
	require.Contains(t, v, "alpha")
	require.Contains(t, v, "beta")
}

func TestPickerCurrentMarker(t *testing.T) {
	t.Parallel()

	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "beta", &selected)
	m.SetSize(60, 20)

	v := m.View()
	require.Contains(t, v, "* beta")
}

func TestPickerNavigation(t *testing.T) {
	t.Parallel()

	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "alpha", &selected)
	m.SetSize(60, 20)

	// Move down.
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(*profileswitch.PickerModel)

	// Select with enter.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, "beta", selected)

	require.NotNil(t, cmd)
	msg := cmd()
	require.IsType(t, overlaywidget.CloseOverlayMsg{}, msg)
}

func TestPickerFilter(t *testing.T) {
	t.Parallel()

	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "alpha", &selected)
	m.SetSize(60, 20)

	// Type "gam" to filter.
	for _, ch := range "gam" {
		model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = model.(*profileswitch.PickerModel)
	}

	v := m.View()
	require.Contains(t, v, "gamma")
	require.NotContains(t, v, "alpha")

	// Enter should select the filtered item.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, "gamma", selected)
}

func TestPickerFilterBackspace(t *testing.T) {
	t.Parallel()

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
	require.Contains(t, v, "alpha")
}

func TestPickerCursorStartsOnCurrentProfile(t *testing.T) {
	t.Parallel()

	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "gamma", &selected)
	m.SetSize(60, 20)

	// Press enter immediately — should select the current profile (gamma), not the first item.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, "gamma", selected)
}

func TestPickerEmptyItems(t *testing.T) {
	t.Parallel()

	var selected string
	m := profileswitch.NewPickerModel(nil, "", &selected)
	m.SetSize(60, 20)

	v := m.View()
	require.Contains(t, v, "No profiles")
}

func TestPickerEnterDoesNotCloseWhenNoItems(t *testing.T) {
	t.Parallel()

	var selected string
	m := profileswitch.NewPickerModel(nil, "", &selected)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	require.Empty(t, selected)
}

func TestPickerEnterDoesNotCloseWhenFilterMatchesNone(t *testing.T) {
	t.Parallel()

	var selected string
	m := profileswitch.NewPickerModel(makeTestItems(), "alpha", &selected)
	m.SetSize(60, 20)

	// Type "xyz" (no matches).
	for _, ch := range "xyz" {
		model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = model.(*profileswitch.PickerModel)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	require.Empty(t, selected)
}

func TestPickerHeightConstraint(t *testing.T) {
	t.Parallel()

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
	require.LessOrEqual(t, len(lines), 12)
}

func TestPickerDisplayNameInLabel(t *testing.T) {
	t.Parallel()

	var selected string
	items := []profileswitch.ProfileListItem{
		{ProfileSlug: gepprofiles.ProfileSlug("test"), DisplayName: "My Test Profile"},
	}
	m := profileswitch.NewPickerModel(items, "", &selected)
	m.SetSize(60, 20)

	v := m.View()
	require.Contains(t, v, "My Test Profile")
}
