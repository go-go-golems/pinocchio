# Building a Credential Editor TUI with Bubbletea and Huh

This tutorial explains how to build a terminal-based user interface (TUI) for viewing and editing credentials. We'll use the Bubbletea framework for the TUI and the Huh library for forms.

## Table of Contents

1. [Application Overview](#application-overview)
2. [Required Packages](#required-packages)
3. [Data Structure](#data-structure)
4. [Application Structure](#application-structure)
5. [Split-Pane Layout](#split-pane-layout)
6. [Modal Overlays](#modal-overlays)
7. [Form Integration](#form-integration)
8. [Custom List Item Styling](#custom-list-item-styling)
9. [Putting It All Together](#putting-it-all-together)

## Application Overview

The Credential Editor is a terminal application that allows users to:

- Browse a list of credentials with masked values
- View detailed information about a credential
- View the actual credential value in a modal overlay
- Edit credential values with a form interface
- Filter credentials by name

The application uses a split-pane layout with a list on the left and details on the right. When a user selects a credential, its information is displayed in the right pane. Pressing 'v' opens a modal overlay showing the actual credential value, and pressing Enter opens a form to edit the credential.

## Required Packages

Our application requires several packages from the Charm ecosystem:

```go
import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)
```

Each package serves a specific purpose:

- `fmt`: Standard formatting package for string formatting
- `io`: Provides basic interfaces for I/O primitives
- `strings`: String manipulation utilities
- `bubbles/list`: Provides a list component for displaying credentials
- `bubbles/viewport`: Provides a scrollable viewport for displaying content
- `bubbletea`: The main framework for building terminal applications
- `huh`: A library for building interactive forms
- `lipgloss`: A styling library for terminal applications
- `log`: A logging library
- `bubbletea-overlay`: A library for creating modal overlays

## Data Structure

We'll start by defining a `Credential` struct to represent our data:

```go
// Credential represents a credential with a name and value
type Credential struct {
	name  string
	value string
}

// Implement the list.Item interface
func (c Credential) Title() string       { return fmt.Sprintf("%s = %s", c.name, maskValue(c.value)) }
func (c Credential) Description() string { return "Press Enter to edit, 'v' to view" }
func (c Credential) FilterValue() string { return c.name }

// Compare returns true if this credential is equal to another credential
func (c Credential) Compare(other Credential) bool {
	return c.name == other.name
}

// Format returns formatted credential information for the side panel
func (c Credential) Format() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Credential: %s\n\n", c.name))
	sb.WriteString(fmt.Sprintf("Value: %s\n\n", maskValue(c.value)))
	sb.WriteString("Press Enter to edit this credential\n")
	sb.WriteString("Press 'v' to view the actual value\n")
	return sb.String()
}

// FormatDetailed returns the credential with the actual value visible
func (c Credential) FormatDetailed() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Credential: %s\n\n", c.name))
	sb.WriteString(fmt.Sprintf("Value: %s\n\n", c.value))
	sb.WriteString("Press Esc to go back\n")
	return sb.String()
}

// maskValue masks the credential value with asterisks
func maskValue(value string) string {
	if len(value) <= 6 {
		return "XXX*****XXX"
	}
	return "XXX*****XXX"
}
```

The `Credential` struct implements the `list.Item` interface, which requires `Title()`, `Description()`, and `FilterValue()` methods. We also add methods for formatting the credential information and comparing credentials.

## Application Structure

Our application follows the Model-View-Update (MVU) architecture pattern. We define a `model` struct to hold the application state:

```go
// Modes for the application
const (
	normalMode = iota
	modalMode
	formMode
)

// Model represents the application state
type model struct {
	list          list.Model
	viewport      viewport.Model
	modalViewport viewport.Model
	overlayModel  *overlay.Model
	formOverlay   *overlay.Model
	credentials   []Credential
	selected      Credential
	newValue      string
	ready         bool
	width         int
	height        int
	err           error
	mode          int
}
```

The model includes:
- `list`: A list component for displaying credentials
- `viewport`: A viewport for displaying credential details
- `modalViewport`: A viewport for displaying the modal content
- `overlayModel`: An overlay for displaying the credential value
- `formOverlay`: An overlay for displaying the edit form
- `credentials`: A slice of credentials
- `selected`: The currently selected credential
- `newValue`: The new value for the credential being edited
- `ready`: A flag indicating if the application is ready
- `width` and `height`: The dimensions of the terminal
- `err`: An error, if any
- `mode`: The current mode of the application (normal, modal, or form)

## Split-Pane Layout

We create a split-pane layout with a list on the left and details on the right:

```go
// baseView renders the base view with the list and details
func (m model) baseView() string {
	listContent := listPane.Width(40).Render(m.list.View())

	var infoContent string
	if (m.selected != Credential{}) {
		infoContent = infoPane.
			Width(m.width - 45).
			Height(m.height - 4).
			Render(m.viewport.View())
	} else {
		infoContent = infoPane.
			Width(m.width - 45).
			Height(m.height - 4).
			Render(noSelectionStyle.Render("Select a credential to view details"))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, listContent, infoContent)
}
```

We use `lipgloss` to style the panes and `JoinHorizontal` to combine them.

## Modal Overlays

We use the `overlay` package to create modal overlays for viewing credential values and editing credentials:

```go
// Show the actual value in a modal
if item, ok := m.list.SelectedItem().(Credential); ok {
	m.selected = item
	m.modalViewport.SetContent(m.selected.FormatDetailed())
	m.modalViewport.GotoTop()

	// Create modal content
	modalWidth := m.width - 20
	modalHeight := m.height - 10
	modal := modalStyle.
		Width(modalWidth).
		Height(modalHeight).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			modalTitleStyle.Render(" Credential Details "),
			m.modalViewport.View(),
			modalCloseHelpStyle.Render("Press ESC to close"),
		))

	// Create overlay model
	m.overlayModel = overlay.New(
		&modalModel{content: modal},
		&baseModel{view: m.baseView()},
		overlay.Center,
		overlay.Center,
		0,
		0,
	)

	m.mode = modalMode
}
```

The `overlay.New` function takes:
1. A model for the overlay content
2. A model for the base view
3. Horizontal and vertical positioning
4. Horizontal and vertical offsets

## Form Integration

We use the `huh` library to create a form for editing credentials:

```go
// Show the edit form in a modal
if item, ok := m.list.SelectedItem().(Credential); ok {
	m.selected = item
	m.newValue = item.value

	// Create the form
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("New value for "+item.name).
				Value(&m.newValue),
		),
	).WithTheme(huh.ThemeCharm())

	// Create form content
	formWidth := m.width - 20
	formHeight := m.height - 10
	formContent := formStyle.
		Width(formWidth).
		Height(formHeight).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			formTitleStyle.Render(" Edit Credential "),
			form.View(),
		))

	// Create overlay model with the form
	m.formOverlay = overlay.New(
		&formModel{
			form:    form,
			content: formContent,
			model:   &m,
		},
		&baseModel{view: m.baseView()},
		overlay.Center,
		overlay.Center,
		0,
		0,
	)

	m.mode = formMode
}
```

The `huh.NewForm` function creates a form with a single input field. We use `WithTheme` to apply the Charm theme to the form.

## Custom List Item Styling

To style list items, especially selected items, we need to create a custom delegate that implements the `list.ItemDelegate` interface. This approach allows us to control exactly how each item is rendered:

```go
// Custom delegate for styling list items
type customDelegate struct {
	list.DefaultDelegate
}

// Render renders a list item with custom styling for selected items
func (d customDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	cred, ok := item.(Credential)
	if !ok {
		return
	}

	// Get the item's title and description
	title := cred.Title()
	desc := cred.Description()

	// Check if this item is selected
	if index == m.Index() {
		// Render selected item with custom styling
		fmt.Fprint(w, selectedItemStyle.Render(title))
		fmt.Fprintf(w, "\n%s", selectedItemStyle.Render(desc))
	} else {
		// Use the default delegate's rendering for non-selected items
		d.DefaultDelegate.Render(w, m, index, item)
	}
}
```

The key points about custom delegates:

1. Create a struct that embeds `list.DefaultDelegate` to inherit default behavior
2. Implement the `Render` method with the signature `Render(w io.Writer, m list.Model, index int, item list.Item)`
3. Check if the item is selected by comparing `index` with `m.Index()`
4. Apply custom styling to selected items
5. Use the default delegate's rendering for non-selected items

When creating the list, use the custom delegate:

```go
// Create a custom delegate for styling list items
delegate := customDelegate{
	DefaultDelegate: list.NewDefaultDelegate(),
}

// Create the list with the custom delegate
l := list.New(items, delegate, 0, 0)
```

This approach gives you complete control over how list items are rendered, allowing for custom styling of selected items.

## Putting It All Together

The main function initializes the application:

```go
func main() {
	// Load credentials
	credentials := loadCredentials()

	// Create list items
	items := make([]list.Item, len(credentials))
	for i, cred := range credentials {
		items[i] = cred
	}

	// Create a custom delegate for styling list items
	delegate := customDelegate{
		DefaultDelegate: list.NewDefaultDelegate(),
	}
	
	// Create the list with the custom delegate
	l := list.New(items, delegate, 0, 0)
	l.Title = "Credentials"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	// Create the model
	m := model{
		list:        l,
		credentials: credentials,
		mode:        normalMode,
	}

	// Run the program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal("Error running program:", err)
	}
}
```

We load the credentials, create list items, initialize the list with a custom delegate, create the model, and run the program.

## Conclusion

This tutorial covered the key components and patterns used in the Credential Editor application. By understanding these patterns, you can build similar applications for viewing and editing different types of data.

Key takeaways:
1. Use the Bubbletea framework for building terminal applications
2. Use the Huh library for building interactive forms
3. Use the Overlay package for creating modal overlays
4. Use the Lipgloss library for styling terminal applications
5. Follow the Model-View-Update (MVU) architecture pattern
6. Create a split-pane layout with a list and details
7. Implement a custom delegate for styling list items
8. Handle different modes for normal, modal, and form views 