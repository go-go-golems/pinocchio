package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mitchellh/go-homedir"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"github.com/spf13/cobra"
)

const (
	// UI constants
	listWidth = 80 // Width of the list pane
)

var (
	// Styling
	titleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("#FFFDF5"))
	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("#FFFDF5")).
				Background(lipgloss.Color("62"))
	paginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle       = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	docStyle        = lipgloss.NewStyle().Margin(1, 2)
	infoTitleStyle  = lipgloss.NewStyle().Bold(true).Underline(true).MarginBottom(1).Foreground(lipgloss.Color("#FFFDF5"))
	infoStyle       = lipgloss.NewStyle().MarginLeft(2)
	infoKeyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#AFAFAF"))
	infoValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5"))

	// Panel styles
	listPane = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	infoPane = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	noSelectionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Align(lipgloss.Center).
				PaddingTop(2)

	// Modal styles
	modalStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("170")).
			Padding(1, 3).
			BorderBottom(true).
			BorderLeft(true).
			BorderRight(true).
			BorderTop(true)

	modalTitleStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	modalCloseHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Align(lipgloss.Center).
				Padding(1, 0)

	// Fixed width styles for list items to prevent wrapping issues
	fixedWidthTitleStyle = lipgloss.NewStyle().
				Width(listWidth - 10).
				Foreground(lipgloss.Color("#FFFDF5"))

	fixedWidthDescStyle = lipgloss.NewStyle().
				Width(listWidth - 10).
				Foreground(lipgloss.Color("#AFAFAF"))

	selectedFixedWidthTitleStyle = fixedWidthTitleStyle.
					Background(lipgloss.Color("62"))

	selectedFixedWidthDescStyle = fixedWidthDescStyle.
					Background(lipgloss.Color("62"))
)

// View modes
const (
	normalMode = iota
	modalMode
)

// HistoryFile interface defines methods for history file information
type HistoryFile interface {
	// Implement list.Item interface
	Title() string
	Description() string
	FilterValue() string

	// Additional methods for file info
	GetPath() string
	GetFullPath() string
	GetTimestamp() time.Time
	GetUUID() string
	GetSize() int64
	GetModTime() time.Time
	GetContent() map[string]interface{}

	// Format information for display
	Format() string

	// Format detailed information for modal display
	FormatDetailed() string

	// Compare returns true if this file is equal to another file
	Compare(other HistoryFile) bool
}

// HistoryFileImpl implements the HistoryFile interface
type HistoryFileImpl struct {
	path      string // Relative path
	fullPath  string // Full path
	filename  string
	title     string
	timestamp time.Time
	uuid      string
	size      int64
	modTime   time.Time
	content   map[string]interface{}
}

// list.Item interface implementation
func (h HistoryFileImpl) Title() string       { return h.title }
func (h HistoryFileImpl) Description() string { return h.path }
func (h HistoryFileImpl) FilterValue() string { return h.path }

// HistoryFile interface implementation
func (h HistoryFileImpl) GetPath() string                    { return h.path }
func (h HistoryFileImpl) GetFullPath() string                { return h.fullPath }
func (h HistoryFileImpl) GetTimestamp() time.Time            { return h.timestamp }
func (h HistoryFileImpl) GetUUID() string                    { return h.uuid }
func (h HistoryFileImpl) GetSize() int64                     { return h.size }
func (h HistoryFileImpl) GetModTime() time.Time              { return h.modTime }
func (h HistoryFileImpl) GetContent() map[string]interface{} { return h.content }

// Format returns formatted file information for display in the side panel
func (h HistoryFileImpl) Format() string {
	var sb strings.Builder

	sb.WriteString(infoTitleStyle.Render("File Information"))
	sb.WriteString("\n\n")

	// Basic file info
	sb.WriteString(infoKeyStyle.Render("Path: "))
	sb.WriteString(infoValueStyle.Render(h.path))
	sb.WriteString("\n")

	sb.WriteString(infoKeyStyle.Render("Size: "))
	sb.WriteString(infoValueStyle.Render(fmt.Sprintf("%d bytes", h.size)))
	sb.WriteString("\n")

	sb.WriteString(infoKeyStyle.Render("Last Modified: "))
	sb.WriteString(infoValueStyle.Render(h.modTime.Format(time.RFC1123)))
	sb.WriteString("\n")

	if !h.timestamp.IsZero() {
		sb.WriteString(infoKeyStyle.Render("Timestamp: "))
		sb.WriteString(infoValueStyle.Render(h.timestamp.Format(time.RFC1123)))
		sb.WriteString("\n")
	}

	if h.uuid != "" {
		sb.WriteString(infoKeyStyle.Render("UUID: "))
		sb.WriteString(infoValueStyle.Render(h.uuid))
		sb.WriteString("\n")
	}

	// Content summary - show just a few key fields for the side panel
	if len(h.content) > 0 {
		sb.WriteString("\n")
		sb.WriteString(infoTitleStyle.Render("Content Preview"))
		sb.WriteString("\n\n")

		// Only show a few fields in the side panel
		importantFields := []string{"command", "type", "name"}
		for _, field := range importantFields {
			if value, exists := h.content[field]; exists {
				sb.WriteString(infoKeyStyle.Render(fmt.Sprintf("%s: ", field)))
				sb.WriteString(infoValueStyle.Render(fmt.Sprintf("%v", value)))
				sb.WriteString("\n")
			}
		}

		sb.WriteString("\n")
		sb.WriteString(infoStyle.Render("Press Enter for more details"))
	}

	return sb.String()
}

// FormatDetailed returns a comprehensive view of the file for modal display
func (h HistoryFileImpl) FormatDetailed() string {
	var sb strings.Builder

	sb.WriteString(infoTitleStyle.Render("Complete File Information"))
	sb.WriteString("\n\n")

	// Basic file info
	sb.WriteString(infoTitleStyle.Render("Metadata"))
	sb.WriteString("\n\n")

	sb.WriteString(infoKeyStyle.Render("Full Path: "))
	sb.WriteString(infoValueStyle.Render(h.fullPath))
	sb.WriteString("\n")

	sb.WriteString(infoKeyStyle.Render("Relative Path: "))
	sb.WriteString(infoValueStyle.Render(h.path))
	sb.WriteString("\n")

	sb.WriteString(infoKeyStyle.Render("Filename: "))
	sb.WriteString(infoValueStyle.Render(h.filename))
	sb.WriteString("\n")

	sb.WriteString(infoKeyStyle.Render("Size: "))
	sb.WriteString(infoValueStyle.Render(fmt.Sprintf("%d bytes", h.size)))
	sb.WriteString("\n")

	sb.WriteString(infoKeyStyle.Render("Last Modified: "))
	sb.WriteString(infoValueStyle.Render(h.modTime.Format(time.RFC1123)))
	sb.WriteString("\n")

	if !h.timestamp.IsZero() {
		sb.WriteString(infoKeyStyle.Render("Timestamp: "))
		sb.WriteString(infoValueStyle.Render(h.timestamp.Format(time.RFC1123)))
		sb.WriteString("\n")
	}

	if h.uuid != "" {
		sb.WriteString(infoKeyStyle.Render("UUID: "))
		sb.WriteString(infoValueStyle.Render(h.uuid))
		sb.WriteString("\n")
	}

	// Full content details
	if len(h.content) > 0 {
		sb.WriteString("\n")
		sb.WriteString(infoTitleStyle.Render("Complete Content"))
		sb.WriteString("\n\n")

		// Show all fields in the modal view
		for field, value := range h.content {
			sb.WriteString(infoKeyStyle.Render(fmt.Sprintf("%s: ", field)))

			// Format the value based on type
			switch v := value.(type) {
			case map[string]interface{}:
				sb.WriteString(infoValueStyle.Render("(Object)"))
				sb.WriteString("\n")
				for k, subVal := range v {
					sb.WriteString(infoStyle.Render(fmt.Sprintf("  %s: %v\n", k, subVal)))
				}
			case []interface{}:
				sb.WriteString(infoValueStyle.Render(fmt.Sprintf("(Array with %d items)", len(v))))
				sb.WriteString("\n")
				for i, item := range v {
					if i < 5 { // Only show first 5 items to avoid overwhelming display
						sb.WriteString(infoStyle.Render(fmt.Sprintf("  %d: %v\n", i, item)))
					}
				}
				if len(v) > 5 {
					sb.WriteString(infoStyle.Render(fmt.Sprintf("  ... %d more items\n", len(v)-5)))
				}
			default:
				sb.WriteString(infoValueStyle.Render(fmt.Sprintf("%v", value)))
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// Compare returns true if this file is equal to another file
func (h HistoryFileImpl) Compare(other HistoryFile) bool {
	if other == nil {
		return false
	}
	return h.GetFullPath() == other.GetFullPath()
}

// Helper function to create a title from a path
func createTitle(path string) string {
	// Extract information from the filename
	filename := filepath.Base(path)
	if filepath.Ext(filename) != ".json" {
		return filename // Return as is if not a JSON file
	}

	// Format: timestamp-uuid.json
	parts := strings.Split(strings.TrimSuffix(filename, ".json"), "-")
	if len(parts) < 2 {
		return filename // Return as is if doesn't match expected format
	}

	// Parse timestamp (format: HHMMSS)
	timestamp := parts[0]
	var timeStr string
	if len(timestamp) == 6 {
		// Try to parse the timestamp
		t, err := time.Parse("150405", timestamp)
		if err == nil {
			timeStr = t.Format("15:04:05")
		} else {
			timeStr = timestamp
		}
	} else {
		timeStr = timestamp
	}

	// Get the date from the directory
	dir := filepath.Dir(path)
	dateDir := filepath.Base(dir)

	return fmt.Sprintf("%s - %s", dateDir, timeStr)
}

// Create a new HistoryFile from a path
func NewHistoryFile(rootDir, path string) (HistoryFile, error) {
	fullPath := filepath.Join(rootDir, path)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	// Create basic history file
	h := HistoryFileImpl{
		path:     path,
		fullPath: fullPath,
		filename: fileInfo.Name(),
		title:    createTitle(fullPath),
		size:     fileInfo.Size(),
		modTime:  fileInfo.ModTime(),
		content:  make(map[string]interface{}),
	}

	// Extract more information if it's a JSON file
	if filepath.Ext(h.filename) == ".json" {
		// Format: timestamp-uuid.json
		parts := strings.Split(strings.TrimSuffix(h.filename, ".json"), "-")
		if len(parts) >= 2 {
			// Parse timestamp (format: HHMMSS)
			timestamp := parts[0]
			if len(timestamp) == 6 {
				// Try to parse the timestamp
				t, err := time.Parse("150405", timestamp)
				if err == nil {
					// Get the date from the directory
					dir := filepath.Dir(fullPath)
					dateDir := filepath.Base(dir)

					// Try to parse the date (format: YYYY/MM/DD)
					if dateStr, err := time.Parse("02", dateDir); err == nil {
						// Create a full timestamp by combining date and time
						year, month, _ := time.Now().Date()
						fullTime := time.Date(year, month, dateStr.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
						h.timestamp = fullTime
					} else {
						h.timestamp = t
					}
				}
			}

			// Extract UUID
			h.uuid = strings.Join(parts[1:], "-")
		}

		// Try to read content
		file, err := os.Open(fullPath)
		if err == nil {
			defer func() {
				_ = file.Close()
			}()
			// Read file content as JSON
			var jsonData map[string]interface{}
			if err := json.NewDecoder(file).Decode(&jsonData); err == nil {
				h.content = jsonData
			}
		}
	}

	return h, nil
}

// Find all history files in a directory
func findHistoryFiles(rootDir string) ([]list.Item, error) {
	var items []list.Item

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Make the path relative to rootDir
			relPath, err := filepath.Rel(rootDir, path)
			if err != nil {
				return err
			}

			// Create history file
			historyFile, err := NewHistoryFile(rootDir, relPath)
			if err != nil {
				return err
			}

			items = append(items, historyFile)
		}
		return nil
	})

	return items, err
}

// Model for the application
type model struct {
	list          list.Model
	viewport      viewport.Model
	modalViewport viewport.Model
	overlayModel  *overlay.Model
	rootDir       string
	selected      HistoryFile
	ready         bool
	width         int
	height        int
	err           error
	mode          int
}

// SelectionChangedMsg is sent when a selection changes in the list
type SelectionChangedMsg struct {
	Selected HistoryFile
}

func (m model) Init() tea.Cmd {
	return nil
}

// baseView returns the normal mode view (split-pane layout)
func (m model) baseView() string {
	// Render the list pane
	listContent := listPane.Width(listWidth).Render(m.list.View())

	// Render the info pane
	var infoContent string
	if m.selected != nil {
		infoContent = infoPane.
			Width(m.width - listWidth - 5).
			Height(m.height - 4).
			Render(m.viewport.View())
	} else {
		infoContent = infoPane.
			Width(m.width - listWidth - 5).
			Height(m.height - 4).
			Render(noSelectionStyle.Render("Select a file to view details"))
	}

	// Combine the panes horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, listContent, infoContent)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case normalMode:
			switch keypress := msg.String(); keypress {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "enter":
				// Show detailed info in modal when enter is pressed
				if m.selected != nil {
					m.mode = modalMode
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
							modalTitleStyle.Render(" File Details "),
							m.modalViewport.View(),
							modalCloseHelpStyle.Render("Press ESC or Enter to close"),
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
					return m, nil
				}
			}

			// First try to handle list updates to catch selection changes
			var listCmd tea.Cmd
			newList, listCmd := m.list.Update(msg)

			// Get old and new selections
			oldSelected := m.list.SelectedItem()
			m.list = newList
			newSelected := m.list.SelectedItem()

			// Check if selection changed using our Compare method
			var changed bool
			if oldHistoryFile, ok := oldSelected.(HistoryFile); ok {
				if newHistoryFile, ok := newSelected.(HistoryFile); ok {
					changed = !oldHistoryFile.Compare(newHistoryFile)
				} else {
					changed = true
				}
			} else if newSelected != nil {
				// Old selection was nil, but new isn't
				changed = true
			}

			if changed {
				if newHistoryFile, ok := newSelected.(HistoryFile); ok {
					m.selected = newHistoryFile
					m.viewport.SetContent(m.selected.Format())
					m.viewport.GotoTop()
				}
			}

			cmds = append(cmds, listCmd)

		case modalMode:
			switch keypress := msg.String(); keypress {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc", "enter", "backspace":
				// Return to normal mode
				m.mode = normalMode
				m.overlayModel = nil
				return m, nil
			}

			// Handle modal viewport scrolling
			var viewportCmd tea.Cmd
			m.modalViewport, viewportCmd = m.modalViewport.Update(msg)
			cmds = append(cmds, viewportCmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			// Set up list in the left pane
			top, right, bottom, left := docStyle.GetMargin()
			m.list.SetSize(listWidth, m.height-top-bottom-3)

			// Set up viewport in the right pane for file details
			m.viewport = viewport.New(m.width-listWidth-left-right-5, m.height-top-bottom-2)
			m.viewport.Style = lipgloss.NewStyle().Padding(0, 1)

			// Set up modal viewport
			modalWidth := m.width - 20
			modalHeight := m.height - 10
			m.modalViewport = viewport.New(modalWidth-4, modalHeight-6)
			m.modalViewport.Style = lipgloss.NewStyle().Padding(0, 0)

			// Initialize with the first item selected (if available)
			if len(m.list.Items()) > 0 {
				if historyFile, ok := m.list.SelectedItem().(HistoryFile); ok {
					m.selected = historyFile
					m.viewport.SetContent(m.selected.Format())
				}
			}

			m.ready = true
		} else {
			// Update sizes when window changes
			top, right, bottom, left := docStyle.GetMargin()
			m.list.SetSize(listWidth, m.height-top-bottom)
			m.viewport.Width = m.width - listWidth - left - right - 5
			m.viewport.Height = m.height - top - bottom - 2

			// Update modal size
			modalWidth := m.width - 20
			modalHeight := m.height - 10
			m.modalViewport.Width = modalWidth - 4
			m.modalViewport.Height = modalHeight - 6
		}
	}

	// In normal mode, ensure viewport gets updated
	if m.mode == normalMode {
		var viewportCmd tea.Cmd
		m.viewport, viewportCmd = m.viewport.Update(msg)
		cmds = append(cmds, viewportCmd)
	}

	return m, tea.Batch(cmds...)
}

// Helper models for overlay
type modalModel struct {
	content string
}

func (m *modalModel) Init() tea.Cmd { return nil }
func (m *modalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}
func (m *modalModel) View() string { return m.content }

type baseModel struct {
	view string
}

func (m *baseModel) Init() tea.Cmd { return nil }
func (m *baseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}
func (m *baseModel) View() string { return m.view }

func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	// Choose view based on mode
	switch m.mode {
	case normalMode:
		return m.baseView()
	case modalMode:
		if m.overlayModel != nil {
			return m.overlayModel.View()
		}
		return m.baseView()
	default:
		return "Unknown view mode"
	}
}

func runTUI(rootDir string) (string, error) {
	// Get absolute path
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("error getting absolute path: %v", err)
	}

	// Find all history files
	items, err := findHistoryFiles(absRootDir)
	if err != nil {
		return "", fmt.Errorf("error finding history files: %v", err)
	}

	if len(items) == 0 {
		return "", fmt.Errorf("no history files found in %s", absRootDir)
	}

	// Create custom delegate for list with fixed width styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = selectedItemStyle
	delegate.Styles.NormalTitle = fixedWidthTitleStyle
	delegate.Styles.NormalDesc = fixedWidthDescStyle
	delegate.Styles.SelectedTitle = selectedFixedWidthTitleStyle
	delegate.Styles.SelectedDesc = selectedFixedWidthDescStyle

	// Prevent title wrapping by using shorter titles in the delegate

	// Set up the list
	l := list.New(items, delegate, 0, 0)
	l.Title = "History Browser"
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle
	// Make sure items display correctly in the list
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)

	// Set up model
	m := model{
		list:    l,
		rootDir: absRootDir,
		mode:    normalMode,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running program: %v", err)
	}

	// Get the selected file
	if m, ok := finalModel.(model); ok && m.selected != nil {
		return m.selected.GetFullPath(), nil
	}

	return "", nil // No selection was made
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "history-browser",
		Short: "A TUI for browsing Pinocchio history recordings",
		Long: `A terminal user interface (TUI) built with bubbletea for browsing 
Pinocchio history recording files in a directory structure.

Files are expected to be organized in directories by date, with filenames 
in the format of TIMESTAMP-UUID.json.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Get directory path from flag
			dirPath, _ := cmd.Flags().GetString("dir")

			// Expand ~ to home directory if present
			expandedPath, err := homedir.Expand(dirPath)
			if err != nil {
				fmt.Printf("Error expanding path: %v\n", err)
				os.Exit(1)
			}

			// Run the TUI
			selectedFile, err := runTUI(expandedPath)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			// If a file was selected, print it (this only happens when TUI exits normally)
			if selectedFile != "" {
				fmt.Printf("Selected file: %s\n", selectedFile)
			}
		},
	}

	// Add flags
	defaultHistoryPath, _ := homedir.Expand("~/.pinocchio/history")
	rootCmd.Flags().StringP("dir", "d", defaultHistoryPath, "Directory containing history recordings")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
