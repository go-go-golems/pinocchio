---
Title: Autosaving Conversations in Pinocchio
Slug: autosave
Short: Configure automatic saving of conversation histories
Topics:
- configuration
- persistence
- history
Commands:
- pinocchio
Flags:
- autosave
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

Pinocchio can automatically save your conversation histories to persistent storage. This allows you to review past conversations and maintain a record of your interactions.

## Configuration

### Command Line

Set autosave options using the `--autosave` flag with key-value pairs:

```bash
pinocchio --autosave enabled:yes,path:/custom/path,template:{{.Time.Format "150405"}}-{{.ConversationID}}.json
```

Available options:
- `enabled`: "yes" or "no" (default: "no")
- `path`: Directory path string for save location (default: `~/.pinocchio/history`)
- `template`: Custom template pattern for filename (default: `{{.Year}}/{{.Month}}/{{.Day}}/{{.Time.Format "150405"}}-{{.ConversationID}}.json`)

### Configuration File

Add to `~/.pinocchio/config.yaml`:

```yaml
autosave:
  enabled: yes
  path: /custom/path/to/history  # optional
  template: custom-template      # optional
```

## Path Templates

The save path supports Go template syntax with these variables:
- `{{.Year}}`: Year (YYYY)
- `{{.Month}}`: Month (MM)
- `{{.Day}}`: Day (DD)
- `{{.Time}}`: Conversation start time
- `{{.ConversationID}}`: Unique conversation identifier

## Default Behavior

When enabled without a custom path:
- Creates `~/.pinocchio/history` directory
- Organizes files by date: `YYYY/MM/DD`
- Names files with timestamp and conversation ID
- Saves as JSON format

## File Structure

Each save file contains:
- Complete conversation history
- Message timestamps
- Conversation tree structure
- Unique conversation identifier

This allows for future browsing, searching, and analysis of conversation histories.