package formoverlay

import (
	"github.com/charmbracelet/huh"
)

// NewSelect creates a FormOverlay with a single Select field.
// onSelect receives the chosen value when the user confirms.
func NewSelect(title string, options []huh.Option[string], onSelect func(value string)) *FormOverlay {
	var selected string
	return New(Config{
		Title: title,
		Factory: func() *huh.Form {
			selected = ""
			return huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title(title).
						Options(options...).
						Value(&selected),
				),
			)
		},
		OnSubmit: func(_ *huh.Form) {
			if onSelect != nil {
				onSelect(selected)
			}
		},
	})
}

// NewConfirm creates a FormOverlay with a single Confirm field.
// onConfirm receives the boolean result when the user confirms.
func NewConfirm(title string, onConfirm func(confirmed bool)) *FormOverlay {
	var confirmed bool
	return New(Config{
		Title: title,
		Factory: func() *huh.Form {
			confirmed = false
			return huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title(title).
						Value(&confirmed),
				),
			)
		},
		OnSubmit: func(_ *huh.Form) {
			if onConfirm != nil {
				onConfirm(confirmed)
			}
		},
	})
}

// NewInput creates a FormOverlay with a single text Input field.
// onSubmit receives the entered value when the user confirms.
func NewInput(title string, onSubmit func(value string)) *FormOverlay {
	var value string
	return New(Config{
		Title: title,
		Factory: func() *huh.Form {
			value = ""
			return huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(title).
						Value(&value),
				),
			)
		},
		OnSubmit: func(_ *huh.Form) {
			if onSubmit != nil {
				onSubmit(value)
			}
		},
	})
}
