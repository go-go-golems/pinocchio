package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"github.com/rs/zerolog"
)

// Logger for debugging
var logger zerolog.Logger

func init() {
	// Initialize zerolog logger
	logFile, err := os.OpenFile("/tmp/edit-credentials.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}

	// Configure zerolog to use the file with caller information
	logger = zerolog.New(logFile).With().Timestamp().
		// Caller().
		Logger()
	logger.Info().Msg("Edit Credentials application started")
}

// Modes for the application
const (
	normalMode = iota
	modalMode
	formMode
)

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
	sb.WriteString(fmt.Sprintf("Credential: %s\n", c.name))
	sb.WriteString(fmt.Sprintf("Value: %s\n", maskValue(c.value)))
	sb.WriteString("Press Enter to edit, 'v' to view")
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
		_, _ = fmt.Fprint(w, selectedItemStyle.Render(title))
		_, _ = fmt.Fprintf(w, "\n%s", selectedItemStyle.Render(desc))
	} else {
		// Use the default delegate's rendering for non-selected items
		d.DefaultDelegate.Render(w, m, index, item)
	}
}

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
	mode          int
}

// Init initializes the model
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		logger.Info().
			Int("width", msg.Width).
			Int("height", msg.Height).
			Bool("ready", m.ready).
			Msg("Window size message received")

		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			// Initialize the list - account for borders and padding
			listWidth := 40
			// Subtract more from height to account for borders and title
			m.list.SetWidth(listWidth)
			m.list.SetHeight(m.height - 8)
			logger.Debug().
				Int("listWidth", listWidth).
				Int("listHeight", m.height-8).
				Msg("Initializing list dimensions")

			// Initialize the viewport for the right pane
			// Account for borders and padding in width and height
			m.viewport = viewport.New(m.width-listWidth-10, m.height-8)
			m.viewport.Style = lipgloss.NewStyle().
				Padding(1, 2)
			logger.Debug().
				Int("viewportWidth", m.width-listWidth-10).
				Int("viewportHeight", m.height-8).
				Msg("Initializing viewport dimensions")

			// Initialize the viewport for the modal
			m.modalViewport = viewport.New(m.width-30, m.height-15)
			m.modalViewport.Style = lipgloss.NewStyle().PaddingLeft(2)
			logger.Debug().
				Int("modalViewportWidth", m.width-30).
				Int("modalViewportHeight", m.height-15).
				Msg("Initializing modal viewport dimensions")

			m.ready = true
		} else {
			// Update sizes when window changes
			listWidth := 40
			// Subtract more from height to account for borders and title
			m.list.SetWidth(listWidth)
			m.list.SetHeight(m.height - 8)
			// Account for borders and padding in width and height
			m.viewport.Width = m.width - listWidth - 10
			m.viewport.Height = m.height - 8
			logger.Debug().
				Int("listWidth", listWidth).
				Int("listHeight", m.height-8).
				Int("viewportWidth", m.width-listWidth-10).
				Int("viewportHeight", m.height-8).
				Msg("Updating dimensions on window resize")

			// Update modal viewport size
			m.modalViewport.Width = m.width - 30
			m.modalViewport.Height = m.height - 15
			logger.Debug().
				Int("modalViewportWidth", m.width-30).
				Int("modalViewportHeight", m.height-15).
				Msg("Updating modal viewport dimensions")
		}

	case tea.KeyMsg:
		logger.Debug().
			Str("key", msg.String()).
			Int("mode", m.mode).
			Int("selectedIndex", m.list.Index()).
			Int("itemCount", len(m.list.Items())).
			Msg("Key press received")

		// Handle different modes
		switch m.mode {
		case normalMode:
			switch msg.String() {
			case "q", "ctrl+c":
				logger.Info().Msg("Quit requested")
				return m, tea.Quit

			case "v":
				logger.Debug().
					Int("selectedIndex", m.list.Index()).
					Msg("View credential details requested")
				// Show the actual value in a modal
				if item, ok := m.list.SelectedItem().(Credential); ok {
					m.selected = item
					m.modalViewport.SetContent(m.selected.FormatDetailed())
					m.modalViewport.GotoTop()
					logger.Debug().
						Str("credential", item.name).
						Int("selectedIndex", m.list.Index()).
						Msg("Showing credential details in modal")

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

			case "enter":
				logger.Debug().
					Int("selectedIndex", m.list.Index()).
					Msg("Edit credential requested")
				// Show the edit form in a modal
				if item, ok := m.list.SelectedItem().(Credential); ok {
					m.selected = item
					m.newValue = item.value
					logger.Debug().
						Str("credential", item.name).
						Int("selectedIndex", m.list.Index()).
						Msg("Showing edit form for credential")

					// Create the form
					form := huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("New value for " + item.name).
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
			}

			// First try to handle list updates to catch selection changes
			var listCmd tea.Cmd
			oldIndex := m.list.Index()
			newList, listCmd := m.list.Update(msg)
			newIndex := newList.Index()

			// Log index changes
			if oldIndex != newIndex {
				logger.Debug().
					Int("oldIndex", oldIndex).
					Int("newIndex", newIndex).
					Msg("List index changed")
			}

			// Get old and new selections
			oldSelected := m.list.SelectedItem()
			m.list = newList
			newSelected := m.list.SelectedItem()

			// Log selection information
			if oldCred, ok := oldSelected.(Credential); ok {
				logger.Debug().
					Str("oldSelection", oldCred.name).
					Int("oldIndex", oldIndex).
					Msg("Previous selection")
			} else {
				logger.Debug().
					Int("oldIndex", oldIndex).
					Msg("No previous selection")
			}

			if newCred, ok := newSelected.(Credential); ok {
				logger.Debug().
					Str("newSelection", newCred.name).
					Int("newIndex", newIndex).
					Msg("New selection")
			} else {
				logger.Debug().
					Int("newIndex", newIndex).
					Msg("No new selection")
			}

			// Check if selection changed
			var changed bool
			if oldCred, ok := oldSelected.(Credential); ok {
				if newCred, ok := newSelected.(Credential); ok {
					changed = !oldCred.Compare(newCred)
					logger.Debug().
						Str("oldSelection", oldCred.name).
						Str("newSelection", newCred.name).
						Int("oldIndex", oldIndex).
						Int("newIndex", newIndex).
						Bool("changed", changed).
						Msg("Selection comparison")
				} else {
					changed = true
					logger.Debug().
						Int("oldIndex", oldIndex).
						Int("newIndex", newIndex).
						Msg("Selection changed: new selection is not a Credential")
				}
			} else if newSelected != nil {
				// Old selection was nil, but new isn't
				changed = true
				logger.Debug().
					Int("oldIndex", oldIndex).
					Int("newIndex", newIndex).
					Msg("Selection changed: old selection was nil")
			}

			if changed {
				if newCred, ok := newSelected.(Credential); ok {
					m.selected = newCred
					m.viewport.SetContent(m.selected.Format())
					m.viewport.GotoTop()
					logger.Debug().
						Str("credential", newCred.name).
						Int("index", newIndex).
						Msg("Updated selection and viewport content")
				}
			}

			cmds = append(cmds, listCmd)

		case modalMode:
			switch msg.String() {
			case "esc", "enter":
				logger.Debug().
					Int("selectedIndex", m.list.Index()).
					Msg("Closing modal")
				m.mode = normalMode
				m.overlayModel = nil
			}

			if m.overlayModel != nil {
				newOverlay, overlayCmd := m.overlayModel.Update(msg)
				if o, ok := newOverlay.(*overlay.Model); ok {
					m.overlayModel = o
				}
				cmds = append(cmds, overlayCmd)
			}

		case formMode:
			if m.formOverlay != nil {
				newOverlay, overlayCmd := m.formOverlay.Update(msg)
				if o, ok := newOverlay.(*overlay.Model); ok {
					m.formOverlay = o
				}
				cmds = append(cmds, overlayCmd)
			}
		}

	case credentialUpdatedMsg:
		logger.Info().
			Str("credential", msg.name).
			Int("selectedIndex", m.list.Index()).
			Msg("Credential updated")

		// Update the credential in the list
		for i, cred := range m.credentials {
			if cred.name == msg.name {
				m.credentials[i].value = msg.value
				break
			}
		}

		// Recreate the list with updated credentials
		items := make([]list.Item, len(m.credentials))
		for i, cred := range m.credentials {
			items[i] = cred
		}
		m.list.SetItems(items)

		// Update the selected credential
		if m.selected.name == msg.name {
			m.selected.value = msg.value
			m.viewport.SetContent(m.selected.Format())
			logger.Debug().
				Str("credential", msg.name).
				Int("selectedIndex", m.list.Index()).
				Msg("Updated viewport content after credential update")
		}

		m.mode = normalMode
		m.formOverlay = nil
	}

	// Update the viewport
	var viewportCmd tea.Cmd
	m.viewport, viewportCmd = m.viewport.Update(msg)
	cmds = append(cmds, viewportCmd)

	// Update the modal viewport
	var modalViewportCmd tea.Cmd
	m.modalViewport, modalViewportCmd = m.modalViewport.Update(msg)
	cmds = append(cmds, modalViewportCmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	switch m.mode {
	case normalMode:
		return m.baseView()
	case modalMode:
		if m.overlayModel != nil {
			return m.overlayModel.View()
		}
		return m.baseView()
	case formMode:
		if m.formOverlay != nil {
			return m.formOverlay.View()
		}
		return m.baseView()
	default:
		return m.baseView()
	}
}

// baseView renders the base view with the list and details
func (m model) baseView() string {
	logger.Debug().
		Int("listWidth", 40).
		Int("infoWidth", m.width-45).
		Int("infoHeight", m.height-4).
		Int("selectedIndex", m.list.Index()).
		Int("itemCount", len(m.list.Items())).
		Msg("Rendering base view with dimensions")

	// Render list with fixed width
	listContent := listPane.Width(40).Render(m.list.View())
	listWidth := lipgloss.Width(listContent)
	listHeight := lipgloss.Height(listContent)
	logger.Debug().
		Int("renderedListWidth", listWidth).
		Int("renderedListHeight", listHeight).
		Msg("List content dimensions")

	var infoContent string
	if (m.selected != Credential{}) {
		// Adjust info pane width and height to account for borders and padding
		// and ensure it fits within the window
		infoContent = infoPane.
			Width(m.width - listWidth - 5).
			MaxHeight(m.height - 8).
			Render(m.viewport.View())
		infoWidth := lipgloss.Width(infoContent)
		infoHeight := lipgloss.Height(infoContent)
		logger.Debug().
			Int("viewportWidth", m.viewport.Width).
			Int("viewportHeight", m.viewport.Height).
			Int("contentWidth", lipgloss.Width(m.viewport.View())).
			Int("contentHeight", lipgloss.Height(m.viewport.View())).
			Bool("atTop", m.viewport.AtTop()).
			Bool("atBottom", m.viewport.AtBottom()).
			// Str("content", m.viewport.View()).
			Str("selectedCredential", m.selected.name).
			Int("selectedIndex", m.list.Index()).
			Int("renderedInfoWidth", infoWidth).
			Int("renderedInfoHeight", infoHeight).
			Msg("Rendering info pane with selected credential")
	} else {
		// Adjust info pane width and height to account for borders and padding
		// and ensure it fits within the window
		infoContent = infoPane.
			Width(m.width-listWidth-5).
			Padding(1, 2).
			MaxHeight(m.height - 8).
			Render(noSelectionStyle.Render("Select a credential to view details"))
		infoWidth := lipgloss.Width(infoContent)
		infoHeight := lipgloss.Height(infoContent)
		logger.Debug().
			Int("selectedIndex", m.list.Index()).
			Int("renderedInfoWidth", infoWidth).
			Int("renderedInfoHeight", infoHeight).
			Msg("Rendering info pane with no selection")
	}

	finalView := lipgloss.JoinHorizontal(lipgloss.Top, listContent, infoContent)
	finalWidth := lipgloss.Width(finalView)
	finalHeight := lipgloss.Height(finalView)

	// Ensure the final view doesn't exceed the window dimensions
	if finalHeight > m.height {
		logger.Warn().
			Int("finalHeight", finalHeight).
			Int("windowHeight", m.height).
			Msg("Final view height exceeds window height")
	}

	logger.Debug().
		Int("finalWidth", finalWidth).
		Int("finalHeight", finalHeight).
		Int("windowWidth", m.width).
		Int("windowHeight", m.height).
		Msg("Final joined view dimensions")

	return finalView
}

// credentialUpdatedMsg is sent when a credential is updated
type credentialUpdatedMsg struct {
	name  string
	value string
}

// formModel is a model for the form overlay
type formModel struct {
	form    *huh.Form
	content string
	model   *model
}

func (m *formModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.model.mode = normalMode
			return m, nil
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f

		if m.form.State == huh.StateCompleted {
			// Send a message to update the credential
			return m, func() tea.Msg {
				return credentialUpdatedMsg{
					name:  m.model.selected.name,
					value: m.model.newValue,
				}
			}
		}
	}

	return m, cmd
}

func (m *formModel) View() string {
	return m.content
}

// modalModel is a model for the modal overlay
type modalModel struct {
	content string
}

func (m *modalModel) Init() tea.Cmd {
	return nil
}

func (m *modalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *modalModel) View() string {
	return m.content
}

// baseModel is a model for the base view
type baseModel struct {
	view string
}

func (m *baseModel) Init() tea.Cmd {
	return nil
}

func (m *baseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *baseModel) View() string {
	return m.view
}

// Styling
var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("#FFFDF5"))
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("62"))
	noSelectionStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Italic(true).Align(lipgloss.Center).PaddingTop(2)

	// Fixed width styles for list items to prevent wrapping issues
	fixedWidthTitleStyle = lipgloss.NewStyle().
				Width(35).
				Foreground(lipgloss.Color("#FFFDF5"))

	fixedWidthDescStyle = lipgloss.NewStyle().
				Width(35).
				Foreground(lipgloss.Color("#AFAFAF"))

	selectedFixedWidthTitleStyle = fixedWidthTitleStyle.
					Background(lipgloss.Color("62"))

	selectedFixedWidthDescStyle = fixedWidthDescStyle.
					Background(lipgloss.Color("62"))

	// Panel styles
	listPane = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	infoPane = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	// Modal styles
	modalStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	modalTitleStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("#FFFDF5")).
			Padding(0, 1).
			Bold(true)

	modalCloseHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFDF5")).
				Italic(true).
				Align(lipgloss.Center).
				PaddingTop(1)

	// Form styles
	formStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	formTitleStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("#FFFDF5")).
			Padding(0, 1).
			Bold(true)
)

// loadCredentials loads credentials from a file or environment
// For this example, we'll use mock data
func loadCredentials() []Credential {
	return []Credential{
		{name: "API_KEY", value: "sk-1234567890abcdef"},
		{name: "DATABASE_URL", value: "postgres://user:password@localhost:5432/mydb"},
		{name: "AWS_ACCESS_KEY", value: "AKIAIOSFODNN7EXAMPLE"},
		{name: "AWS_SECRET_KEY", value: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"},
		{name: "GITHUB_TOKEN", value: "ghp_1234567890abcdefghijklmnopqrstuvwxyz"},
		{name: "OPENAI_API_KEY", value: "sk-openai-1234567890abcdefghijklmnopqrstuvwxyz"},
		{name: "SLACK_WEBHOOK", value: "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX"},
		{name: "JWT_SECRET", value: "super-secret-jwt-token-with-at-least-32-characters"},
		{name: "SMTP_PASSWORD", value: "email-password-123"},
		{name: "ENCRYPTION_KEY", value: "AES256-encryption-key-example"},
	}
}

func main() {
	logger.Info().Msg("Starting Edit Credentials application")

	// Load credentials
	credentials := loadCredentials()
	logger.Info().
		Int("credentialCount", len(credentials)).
		Msg("Loaded credentials")

	// Create list items
	items := make([]list.Item, len(credentials))
	for i, cred := range credentials {
		items[i] = cred
	}

	// Create a custom delegate for styling list items
	delegate := customDelegate{
		DefaultDelegate: list.NewDefaultDelegate(),
	}

	// Apply fixed width styles to the delegate
	delegate.Styles.NormalTitle = fixedWidthTitleStyle
	delegate.Styles.NormalDesc = fixedWidthDescStyle
	delegate.Styles.SelectedTitle = selectedFixedWidthTitleStyle
	delegate.Styles.SelectedDesc = selectedFixedWidthDescStyle
	logger.Debug().Msg("Applied fixed width styles to delegate")

	// Create the list with the custom delegate
	l := list.New(items, delegate, 0, 0)
	l.Title = "Credentials"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	logger.Debug().Msg("Created list with custom delegate")

	// Create the model
	m := model{
		list:        l,
		credentials: credentials,
		mode:        normalMode,
	}
	logger.Debug().Msg("Created model")

	// Run the program
	logger.Info().Msg("Starting Bubbletea program")
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		logger.Error().Err(err).Msg("Error running program")
		log.Fatal("Error running program:", err)
	}
	logger.Info().Msg("Edit Credentials application exiting")
}
