# Building a Data Browser TUI with Bubbletea

This tutorial explains how we built the History Browser application, a terminal-based UI for browsing and viewing data. While our implementation focuses on JSON files, the concepts and patterns can be applied to build similar applications for browsing and viewing any type of data - users, products, tasks, or any other structured information.

## Table of Contents

1. [Application Overview](#application-overview)
2. [Bubbletea Structure](#bubbletea-structure)
3. [Data Interface Pattern](#data-interface-pattern)
4. [Split-Pane Layout](#split-pane-layout)
5. [Modal Overlays](#modal-overlays)
6. [Control Flow](#control-flow)
7. [Styling and UI Design](#styling-and-ui-design)
8. [Command-Line Integration](#command-line-integration)

## Application Overview

The History Browser is a terminal user interface (TUI) application that allows users to:

- Browse a collection of data items (in our case, JSON history files)
- View item information in a side panel
- See detailed item content in a modal overlay
- Navigate with keyboard shortcuts

The application uses a split-pane layout with an item list on the left and details on the right. When a user selects an item, its information is displayed in the right pane. Pressing Enter opens a modal overlay with more detailed information.

## Bubbletea Structure

[Bubbletea](https://github.com/charmbracelet/bubbletea) is an Elm-inspired framework for building terminal applications. It follows a Model-View-Update (MVU) architecture:

### Model

The model holds the application state:

```go
type model struct {
    list          list.Model
    viewport      viewport.Model
    modalViewport viewport.Model
    overlayModel  *overlay.Model
    rootDir       string
    selected      HistoryFile // Could be any data type: User, Product, Task, etc.
    ready         bool
    width         int
    height        int
    err           error
    mode          int
}
```

Key components:
- `list.Model`: Manages the item list (from Bubble Tea's list component)
- `viewport.Model`: Manages scrollable content views
- `overlayModel`: Handles the modal overlay
- `selected`: Currently selected item
- `mode`: Current view mode (normal or modal)

### Init

The `Init` function sets up the initial state and returns any commands to run:

```go
func (m model) Init() tea.Cmd {
    return nil
}
```

In our application, we initialize most components when we receive the first window size message.

### Update

The `Update` function handles messages and updates the model:

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Handle different message types
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle key presses
    case tea.WindowSizeMsg:
        // Handle window resizing
    }
    
    return m, cmd
}
```

Key patterns in our update function:
1. Mode-based message handling (normal vs. modal)
2. Delegating updates to sub-components (list, viewport)
3. Detecting selection changes
4. Updating content based on selection

### View

The `View` function renders the UI:

```go
func (m model) View() string {
    // Render based on current mode
    switch m.mode {
    case normalMode:
        return m.baseView()
    case modalMode:
        if m.overlayModel != nil {
            return m.overlayModel.View()
        }
        return m.baseView()
    }
}
```

## Data Interface Pattern

We use a generic interface to abstract data operations and information. In our case, we used `HistoryFile`, but this pattern can be applied to any data type:

```go
// Generic example for any data type
type DataItem interface {
    // list.Item interface (required for Bubbletea's list component)
    Title() string
    Description() string
    FilterValue() string

    // Data access methods
    GetID() string
    GetMetadata() map[string]interface{}
    
    // Formatting methods for different views
    Format() string
    FormatDetailed() string
    
    // Comparison method
    Compare(other DataItem) bool
}
```

For our specific implementation with history files:

```go
type HistoryFile interface {
    // list.Item interface
    Title() string
    Description() string
    FilterValue() string

    // File information
    GetPath() string
    GetFullPath() string
    GetTimestamp() time.Time
    GetUUID() string
    GetSize() int64
    GetModTime() time.Time
    GetContent() map[string]interface{}

    // Formatting
    Format() string
    FormatDetailed() string
    
    // Comparison
    Compare(other HistoryFile) bool
}
```

### Benefits of the Interface Approach

1. **Type Safety**: The interface ensures all data objects implement required methods.
2. **Abstraction**: The application works with any object that implements the interface.
3. **Testability**: We can create mock implementations for testing.
4. **Extensibility**: We can add new data types by implementing the interface.

### Implementation Examples

#### For Files (Our Implementation)

```go
type HistoryFileImpl struct {
    path      string
    fullPath  string
    filename  string
    title     string
    timestamp time.Time
    uuid      string
    size      int64
    modTime   time.Time
    content   map[string]interface{}
}
```

#### For Users (Example)

```go
type User struct {
    id        string
    username  string
    email     string
    fullName  string
    createdAt time.Time
    lastLogin time.Time
    roles     []string
    metadata  map[string]interface{}
}

// Implement list.Item interface
func (u User) Title() string       { return u.username }
func (u User) Description() string { return u.email }
func (u User) FilterValue() string { return u.username }

// Implement data access methods
func (u User) GetID() string                    { return u.id }
func (u User) GetMetadata() map[string]interface{} { return u.metadata }

// Format returns formatted user information for the side panel
func (u User) Format() string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("User: %s\n", u.username))
    sb.WriteString(fmt.Sprintf("Name: %s\n", u.fullName))
    sb.WriteString(fmt.Sprintf("Email: %s\n", u.email))
    // Add more basic information
    return sb.String()
}

// FormatDetailed returns comprehensive user information for the modal
func (u User) FormatDetailed() string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("User ID: %s\n", u.id))
    sb.WriteString(fmt.Sprintf("Username: %s\n", u.username))
    sb.WriteString(fmt.Sprintf("Full Name: %s\n", u.fullName))
    sb.WriteString(fmt.Sprintf("Email: %s\n", u.email))
    sb.WriteString(fmt.Sprintf("Created: %s\n", u.createdAt.Format(time.RFC1123)))
    sb.WriteString(fmt.Sprintf("Last Login: %s\n", u.lastLogin.Format(time.RFC1123)))
    sb.WriteString("Roles:\n")
    for _, role := range u.roles {
        sb.WriteString(fmt.Sprintf("  - %s\n", role))
    }
    // Add more detailed information
    return sb.String()
}

// Compare returns true if this user is equal to another user
func (u User) Compare(other DataItem) bool {
    if otherUser, ok := other.(User); ok {
        return u.id == otherUser.id
    }
    return false
}
```

#### For Products (Example)

```go
type Product struct {
    id          string
    name        string
    description string
    price       float64
    category    string
    inStock     bool
    createdAt   time.Time
    metadata    map[string]interface{}
}

// Similar implementation of interface methods...
```

### Key Methods for Any Data Type

- `Format()`: Returns formatted information for the side panel
- `FormatDetailed()`: Returns comprehensive information for the modal
- `Compare()`: Compares two items for equality

## Split-Pane Layout

The application uses a split-pane layout with a list on the left and details on the right.

### Layout Structure

```
┌─────────────────────┐┌─────────────────────────┐
│                     ││                         │
│     Item List       ││     Item Details        │
│                     ││                         │
│                     ││                         │
│                     ││                         │
│                     ││                         │
│                     ││                         │
└─────────────────────┘└─────────────────────────┘
```

### Implementation

We create the layout in the `baseView` method:

```go
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
            Render(noSelectionStyle.Render("Select an item to view details"))
    }

    // Combine the panes horizontally
    return lipgloss.JoinHorizontal(lipgloss.Top, listContent, infoContent)
}
```

Key techniques:
1. Using `lipgloss` styles to create bordered panes
2. Setting fixed width for the list pane
3. Calculating the right pane width based on window size
4. Using `JoinHorizontal` to combine the panes
5. Handling the case when no item is selected

## Modal Overlays

We use the `bubbletea-overlay` package to create modal overlays that appear on top of the main view.

### Modal Structure

```
┌─────────────────────────────────────────────────┐
│                                                 │
│  ┌─────────────────────────────────────┐        │
│  │ Item Details                         │        │
│  │                                     │        │
│  │ Detailed item information...        │        │
│  │                                     │        │
│  │                                     │        │
│  │                                     │        │
│  │ Press ESC or Enter to close         │        │
│  └─────────────────────────────────────┘        │
│                                                 │
└─────────────────────────────────────────────────┘
```

### Implementation

We create the overlay in the update function when Enter is pressed:

```go
// Create modal content
modalWidth := m.width - 20
modalHeight := m.height - 10
modal := modalStyle.
    Width(modalWidth).
    Height(modalHeight).
    Render(lipgloss.JoinVertical(
        lipgloss.Left,
        modalTitleStyle.Render(" Item Details "),
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
```

Key components:
1. `modalModel`: A simple model that holds the modal content
2. `baseModel`: A model that holds the base view (what's behind the modal)
3. `overlay.New`: Creates a new overlay with the modal on top of the base view
4. Positioning parameters: Center horizontally and vertically

### Helper Models

We need simple models for the overlay:

```go
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
```

## Control Flow

The application uses a mode-based control flow to handle different states.

### Modes

```go
const (
    normalMode = iota
    modalMode
)
```

### Mode-Based Update Logic

```go
switch m.mode {
case normalMode:
    // Handle normal mode updates
    // Process list navigation
    // Detect selection changes
case modalMode:
    // Handle modal mode updates
    // Process modal navigation
    // Handle modal closing
}
```

### Selection Change Detection

A key pattern is detecting when the selected item changes:

```go
// Get old and new selections
oldSelected := m.list.SelectedItem()
m.list = newList
newSelected := m.list.SelectedItem()

// Check if selection changed using our Compare method
var changed bool
if oldItem, ok := oldSelected.(DataItem); ok {
    if newItem, ok := newSelected.(DataItem); ok {
        changed = !oldItem.Compare(newItem)
    } else {
        changed = true
    }
} else if newSelected != nil {
    // Old selection was nil, but new isn't
    changed = true
}

if changed {
    if newItem, ok := newSelected.(DataItem); ok {
        m.selected = newItem
        m.viewport.SetContent(m.selected.Format())
        m.viewport.GotoTop()
    }
}
```

This pattern:
1. Gets the old and new selections
2. Uses type assertions to convert to our interface type
3. Uses the `Compare` method to check for equality
4. Updates the right pane content when the selection changes

## Styling and UI Design

We use `lipgloss` for styling the UI components:

```go
// Styling
titleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("#FFFDF5"))
selectedItemStyle = lipgloss.NewStyle().
            PaddingLeft(2).
            Foreground(lipgloss.Color("#FFFDF5")).
            Background(lipgloss.Color("62"))
// ...

// Panel styles
listPane = lipgloss.NewStyle().
        BorderStyle(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("62")).
        Padding(1, 2)
// ...
```

Key styling techniques:
1. Defining styles at the package level for reuse
2. Using color constants for consistency
3. Creating distinct styles for different states (selected, normal)
4. Using border styles to create visual separation
5. Setting fixed widths to prevent wrapping issues

## Command-Line Integration

We use Cobra to create a command-line interface:

```go
func main() {
    var rootCmd = &cobra.Command{
        Use:   "data-browser",
        Short: "A TUI for browsing data items",
        Long:  `...`,
        Run: func(cmd *cobra.Command, args []string) {
            // Get configuration from flags
            configPath, _ := cmd.Flags().GetString("config")
            
            // Run the TUI
            selectedItem, err := runTUI(configPath)
            // ...
        },
    }

    // Add flags
    rootCmd.Flags().StringP("config", "c", "config.json", "Path to configuration file")

    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
```

This pattern:
1. Creates a Cobra command with description and usage information
2. Adds flags for configuration
3. Sets sensible defaults
4. Runs the TUI and handles the result

## Conclusion

This tutorial covered the key components and patterns used in the History Browser application. By understanding these patterns, you can build similar applications for browsing and viewing different types of data.

Key takeaways:
1. Use interfaces to abstract data operations for any data type
2. Implement the `list.Item` interface for compatibility with Bubbletea's list component
3. Use a mode-based control flow for different application states
4. Create split-pane layouts with `lipgloss`
5. Use the `bubbletea-overlay` package for modal dialogs
6. Detect selection changes with a `Compare` method
7. Use Cobra for command-line integration

These patterns can be adapted for various applications, such as:
- User management interfaces
- Product catalogs
- Task managers
- Configuration editors
- Log viewers
- Database record browsers
- API response explorers

The core pattern remains the same - define an interface for your data, implement the required methods, and use Bubbletea's components to create a responsive and interactive TUI. 