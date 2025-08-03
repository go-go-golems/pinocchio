package cmdlayers

import (
	"os"
	"path/filepath"

	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
)

type AutosaveSettings struct {
	Path     string `glazed.parameter:"path"`
	Template string `glazed.parameter:"template"`
	Enabled  string `glazed.parameter:"enabled"`
}

type HelpersSettings struct {
	PrintPrompt       bool                   `glazed.parameter:"print-prompt"`
	System            string                 `glazed.parameter:"system"`
	AppendMessageFile string                 `glazed.parameter:"append-message-file"`
	MessageFile       string                 `glazed.parameter:"message-file"`
	StartInChat       bool                   `glazed.parameter:"chat"`
	Interactive       bool                   `glazed.parameter:"interactive"`
	ForceInteractive  bool                   `glazed.parameter:"force-interactive"`
	Images            []*parameters.FileData `glazed.parameter:"images"`
	Autosave          *AutosaveSettings      `glazed.parameter:"autosave,from_json"`
	NonInteractive    bool                   `glazed.parameter:"non-interactive"`
	Output            string                 `glazed.parameter:"output"`
	WithMetadata      bool                   `glazed.parameter:"with-metadata"`
	FullOutput        bool                   `glazed.parameter:"full-output"`
	UseStepBackend    bool                   `glazed.parameter:"use-step-backend"`
}

const GeppettoHelpersSlug = "geppetto-helpers"

func NewHelpersParameterLayer() (layers.ParameterLayer, error) {
	defaultHistoryPath := filepath.Join(os.Getenv("HOME"), ".pinocchio", "history")

	return layers.NewParameterLayer(GeppettoHelpersSlug, "Geppetto helpers",
		layers.WithParameterDefinitions(
			parameters.NewParameterDefinition(
				"print-prompt",
				parameters.ParameterTypeBool,
				parameters.WithDefault(false),
				parameters.WithHelp("Print the prompt"),
			),
			parameters.NewParameterDefinition(
				"system",
				parameters.ParameterTypeString,
				parameters.WithHelp("System message"),
			),
			parameters.NewParameterDefinition(
				"append-message-file",
				parameters.ParameterTypeString,
				parameters.WithHelp("File containing messages (json or yaml, list of objects with fields text, time, role) to be appended to the already present list of messages"),
			),
			parameters.NewParameterDefinition(
				"message-file",
				parameters.ParameterTypeString,
				parameters.WithHelp("File containing messages (json or yaml, list of objects with fields text, time, role)"),
			),
			parameters.NewParameterDefinition(
				"interactive",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Ask for chat continuation after inference"),
				parameters.WithDefault(true),
			),
			parameters.NewParameterDefinition(
				"chat",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Start in chat mode"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"force-interactive",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Always enter interactive mode, even with non-tty stdout"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"images",
				parameters.ParameterTypeFileList,
				parameters.WithHelp("Images to display"),
			),
			parameters.NewParameterDefinition(
				"autosave",
				parameters.ParameterTypeKeyValue,
				parameters.WithHelp("Autosave configuration"),
				parameters.WithDefault(map[string]interface{}{
					"path":     defaultHistoryPath,
					"template": "",
					"enabled":  "no",
				}),
			),
			parameters.NewParameterDefinition(
				"non-interactive",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Skip interactive chat mode entirely"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"output",
				parameters.ParameterTypeChoice,
				parameters.WithHelp("Output format (text, json, yaml)"),
				parameters.WithDefault("text"),
				parameters.WithChoices("text", "json", "yaml"),
			),
			parameters.NewParameterDefinition(
				"with-metadata",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Include event metadata in output"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"full-output",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Print all available metadata in output"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"use-step-backend",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Use legacy StepBackend instead of EngineBackend for UI (for testing/compatibility)"),
				parameters.WithDefault(false),
			),
		),
	)
}
