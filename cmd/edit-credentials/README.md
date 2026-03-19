# Credential Editor

A terminal-based user interface (TUI) for viewing and editing credentials.

## Features

- Browse a list of credentials with masked values
- View detailed information about a credential
- View the actual credential value in a modal overlay
- Edit credential values with a form interface
- Filter credentials by name

## Installation

This application requires Go 1.18 or later. To install the required dependencies:

```bash
go get github.com/charmbracelet/bubbles/list
go get github.com/charmbracelet/bubbles/viewport
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/huh
go get github.com/charmbracelet/lipgloss
go get github.com/charmbracelet/log
go get github.com/rmhubbert/bubbletea-overlay
```

## Usage

```bash
go run main.go
```

## Keyboard Shortcuts

- **↑/↓**: Navigate through the credential list
- **Enter**: Edit the selected credential
- **v**: View the actual value of the selected credential
- **Esc**: Close modal overlays
- **q** or **Ctrl+C**: Quit the application
- **/** or **Ctrl+F**: Filter credentials by name

## Implementation Details

This application is built using:

- [Bubble Tea](https://github.com/charmbracelet/bubbletea): A framework for building terminal applications
- [Huh](https://github.com/charmbracelet/huh): A library for building interactive forms and prompts
- [Lipgloss](https://github.com/charmbracelet/lipgloss): A styling library for terminal applications
- [Bubbletea Overlay](https://github.com/rmhubbert/bubbletea-overlay): A library for creating modal overlays

The application follows the Model-View-Update (MVU) architecture pattern and uses a split-pane layout with a list on the left and details on the right. Modal overlays are used for viewing and editing credential values.

## Required Packages

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

## Structure

- **Credential**: Represents a credential with a name and value
- **customDelegate**: Custom delegate for styling list items
- **model**: The main application model
- **formModel**: A model for the form overlay
- **modalModel**: A model for the modal overlay
- **baseModel**: A model for the base view

## Customization

You can customize the application by:

- Modifying the `loadCredentials` function to load credentials from a file or environment
- Changing the styling variables to match your preferred color scheme
- Adding additional fields to the credential form 