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
			// Return a minimal form that shows an error.
			return huh.NewForm(
				huh.NewGroup(
					huh.NewNote().Title("Error").Description("No profiles available."),
				),
			)
		}

		opts := make([]huh.Option[string], 0, len(items))
		for _, it := range items {
			title := it.ProfileSlug.String()
			if strings.TrimSpace(it.DisplayName) != "" {
				title = fmt.Sprintf("%s — %s", it.ProfileSlug.String(), it.DisplayName)
			}
			opts = append(opts, huh.NewOption(title, it.ProfileSlug.String()))
		}

		return huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Switch profile").
					Options(opts...).
					Value(selected),
			),
		)
	}
}
