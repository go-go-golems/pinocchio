package profileswitch

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

// PickerFormFactory returns a factory function that creates a fresh huh.Form
// for picking a profile. The selected slug is written to the provided pointer.
// This is designed for use with formoverlay.Config.Factory.
func PickerFormFactory(mgr *Manager, selected *string) func() *huh.Form {
	return func() *huh.Form {
		items, err := mgr.ListProfiles(context.Background())
		if err != nil || len(items) == 0 {
			return huh.NewForm(
				huh.NewGroup(
					huh.NewNote().Title("Error").Description("No profiles available."),
				),
			)
		}

		current := mgr.Current().ProfileSlug.String()
		opts := make([]huh.Option[string], 0, len(items))
		for _, it := range items {
			label := formatPickerLabel(it, current)
			opts = append(opts, huh.NewOption(label, it.ProfileSlug.String()))
		}

		sel := huh.NewSelect[string]().
			Title("Switch profile").
			Options(opts...).
			Value(selected).
			Height(min(len(opts)+2, 15))

		return huh.NewForm(huh.NewGroup(sel))
	}
}

// formatPickerLabel builds a display label for a profile list item.
// The current active profile is marked with a bullet.
func formatPickerLabel(it ProfileListItem, currentSlug string) string {
	var parts []string

	// Mark current profile.
	slug := it.ProfileSlug.String()
	if slug == currentSlug {
		parts = append(parts, fmt.Sprintf("* %s", slug))
	} else {
		parts = append(parts, fmt.Sprintf("  %s", slug))
	}

	if name := strings.TrimSpace(it.DisplayName); name != "" {
		parts[0] += " — " + name
	}

	if desc := strings.TrimSpace(it.Description); desc != "" {
		// Truncate long descriptions.
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		parts = append(parts, fmt.Sprintf("    %s", desc))
	}

	return strings.Join(parts, "\n")
}
