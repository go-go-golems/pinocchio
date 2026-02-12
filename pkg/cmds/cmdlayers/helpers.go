package cmdlayers

import (
	"os"
	"path/filepath"

	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
)

type AutosaveSettings struct {
	Path     string `glazed:"path"`
	Template string `glazed:"template"`
	Enabled  string `glazed:"enabled"`
}

type HelpersSettings struct {
	PrintPrompt       bool               `glazed:"print-prompt"`
	System            string             `glazed:"system"`
	AppendMessageFile string             `glazed:"append-message-file"`
	MessageFile       string             `glazed:"message-file"`
	StartInChat       bool               `glazed:"chat"`
	Interactive       bool               `glazed:"interactive"`
	ForceInteractive  bool               `glazed:"force-interactive"`
	TimelineDSN       string             `glazed:"timeline-dsn"`
	TimelineDB        string             `glazed:"timeline-db"`
	TurnsDSN          string             `glazed:"turns-dsn"`
	TurnsDB           string             `glazed:"turns-db"`
	Images            []*fields.FileData `glazed:"images"`
	Autosave          *AutosaveSettings  `glazed:"autosave,from_json"`
	NonInteractive    bool               `glazed:"non-interactive"`
	Output            string             `glazed:"output"`
	WithMetadata      bool               `glazed:"with-metadata"`
	FullOutput        bool               `glazed:"full-output"`
}

const GeppettoHelpersSlug = "geppetto-helpers"

func NewHelpersParameterLayer() (schema.Section, error) {
	defaultHistoryPath := filepath.Join(os.Getenv("HOME"), ".pinocchio", "history")

	return schema.NewSection(GeppettoHelpersSlug, "Geppetto helpers",
		schema.WithFields(
			fields.New(
				"print-prompt",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Print the prompt"),
			),
			fields.New(
				"system",
				fields.TypeString,
				fields.WithHelp("System message"),
			),
			fields.New(
				"append-message-file",
				fields.TypeString,
				fields.WithHelp("File containing messages (json or yaml, list of objects with fields text, time, role) to be appended to the already present list of messages"),
			),
			fields.New(
				"message-file",
				fields.TypeString,
				fields.WithHelp("File containing messages (json or yaml, list of objects with fields text, time, role)"),
			),
			fields.New(
				"interactive",
				fields.TypeBool,
				fields.WithHelp("Ask for chat continuation after inference"),
				fields.WithDefault(true),
			),
			fields.New(
				"chat",
				fields.TypeBool,
				fields.WithHelp("Start in chat mode"),
				fields.WithDefault(false),
			),
			fields.New(
				"force-interactive",
				fields.TypeBool,
				fields.WithHelp("Always enter interactive mode, even with non-tty stdout"),
				fields.WithDefault(false),
			),
			fields.New(
				"timeline-dsn",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("SQLite DSN for durable timeline snapshots (preferred over timeline-db)"),
			),
			fields.New(
				"timeline-db",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("SQLite DB file path for durable timeline snapshots (DSN derived with WAL/busy_timeout)"),
			),
			fields.New(
				"turns-dsn",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("SQLite DSN for durable turn snapshots (preferred over turns-db)"),
			),
			fields.New(
				"turns-db",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("SQLite DB file path for durable turn snapshots (DSN derived with WAL/busy_timeout)"),
			),
			fields.New(
				"images",
				fields.TypeFileList,
				fields.WithHelp("Images to display"),
			),
			fields.New(
				"autosave",
				fields.TypeKeyValue,
				fields.WithHelp("Autosave configuration"),
				fields.WithDefault(map[string]interface{}{
					"path":     defaultHistoryPath,
					"template": "",
					"enabled":  "no",
				}),
			),
			fields.New(
				"non-interactive",
				fields.TypeBool,
				fields.WithHelp("Skip interactive chat mode entirely"),
				fields.WithDefault(false),
			),
			fields.New(
				"output",
				fields.TypeChoice,
				fields.WithHelp("Output format (text, json, yaml)"),
				fields.WithDefault("text"),
				fields.WithChoices("text", "json", "yaml"),
			),
			fields.New(
				"with-metadata",
				fields.TypeBool,
				fields.WithHelp("Include event metadata in output"),
				fields.WithDefault(false),
			),
			fields.New(
				"full-output",
				fields.TypeBool,
				fields.WithHelp("Print all available metadata in output"),
				fields.WithDefault(false),
			),
		),
	)
}
