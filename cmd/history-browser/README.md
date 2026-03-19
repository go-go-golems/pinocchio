# History Browser

A charmbracelet bubbletea TUI application for browsing history recordings.

## Features

- Browse through history recording files in a specified directory
- Split-pane interface with file list on the left and details on the right
- Files are displayed with a formatted title based on their timestamp
- Automatically displays selected file information in the side panel
- Detailed modal view for comprehensive file information
- Navigate through the list with keyboard arrows
- View useful metadata and content from selected history files without leaving the list view
- Uses Cobra for command-line argument handling
- Implements a proper `HistoryFile` interface for typed file access

## Usage

```bash
# Run with default directory (~/.pinocchio/history)
go run .

# Specify a custom directory
go run . --dir=/path/to/history/files
# Or with the short flag
go run . -d /path/to/history/files
```

## Command-line Options

- `--dir`, `-d`: Specify the directory containing history recordings (default: ~/.pinocchio/history)

## Navigation

- Arrow keys: Navigate through the list of files
  - File information is automatically displayed in the right panel
- Enter: Open a detailed modal view of the currently selected file
- Up/Down: Scroll through the file details
- ESC/Enter/Backspace: Close the modal view and return to the list
- q or Ctrl+C: Quit the application

## Interface Structure

The application implements a `HistoryFile` interface that provides:

- Standard list.Item methods (Title, Description, FilterValue)
- File metadata access methods (GetPath, GetTimestamp, GetUUID, etc.)
- Content formatting methods for display:
  - Format(): Basic information for side panel
  - FormatDetailed(): Comprehensive information for modal view

## File Information Display

The right panel automatically shows:
- Basic file information (path, size, last modified date)
- Timestamp and UUID extracted from filename
- Preview of the most important content fields

The modal view shows:
- Complete file metadata
- Full hierarchical display of JSON content
- Nested object and array visualization

## File Format

The application expects history files to be in the format:
`TIMESTAMP-UUID.json`

For example:
`095214-488720fa-10be-457a-b677-522a96682ee8.json`

The title displayed in the list is created from:
- The date (extracted from the parent directory name)
- The time (formatted from the timestamp in the filename)

## Examples

For a file at `./2024/12/09/095214-488720fa-10be-457a-b677-522a96682ee8.json`,
the title would be displayed as `09 - 09:52:14` 