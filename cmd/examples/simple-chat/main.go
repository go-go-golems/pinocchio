package main

import (
	"context"
	_ "embed"
	"fmt"
	layers2 "github.com/go-go-golems/geppetto/pkg/layers"
	"io"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/helpers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/go-go-golems/geppetto/pkg/turns"
)

//go:embed test.yaml
var testYaml []byte

var rootCmd = &cobra.Command{
	Use:   "simple-chat-step",
	Short: "A simple chat step",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logging.InitLoggerFromViper()
	},
}

type TestCommand struct {
	*cmds.CommandDescription
	pinocchioCmd *pinocchio_cmds.PinocchioCommand
}

type ChatCommandSettings struct {
	PinocchioProfile string `glazed.parameter:"pinocchio-profile"`
	Debug            bool   `glazed.parameter:"debug"`
	ServerTools      bool   `glazed.parameter:"server-tools"`
}

// NewChatCommand wraps the GepettoCommand which was loaded from the yaml file,
// and manually loads the profile to configure it.
func NewChatCommand(cmd *pinocchio_cmds.PinocchioCommand) (*TestCommand, error) {
	geppettoLayers, err := layers2.CreateGeppettoLayers()
	if err != nil {
		return nil, err
	}
	return &TestCommand{
		CommandDescription: cmds.NewCommandDescription("chat",
			cmds.WithShort("Run chat with simple streaming printer"),
			cmds.WithFlags(
				parameters.NewParameterDefinition("pinocchio-profile",
					parameters.ParameterTypeString,
					parameters.WithHelp("Pinocchio profile"),
					parameters.WithDefault("default"),
				),
				parameters.NewParameterDefinition("debug",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Debug mode"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition("server-tools",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Enable Responses server-side tools (web_search)"),
					parameters.WithDefault(false),
				),
			),
			cmds.WithLayersList(
				geppettoLayers...,
			),
		),
		pinocchioCmd: cmd,
	}, nil
}

func (c *TestCommand) RunIntoWriter(ctx context.Context, parsedLayers *layers.ParsedLayers, w io.Writer) error {
	s := &ChatCommandSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return errors.Wrap(err, "failed to initialize settings")
	}

	geppettoParsedLayers, err := helpers.ParseGeppettoLayers(c.pinocchioCmd, helpers.WithProfile(s.PinocchioProfile))
	if err != nil {
		return err
	}

	if s.Debug {
		// marshal geppettoParsedLayer to yaml and print it
		b_, err := yaml.Marshal(geppettoParsedLayers)
		if err != nil {
			return err
		}
		fmt.Println(string(b_))
		return nil
	}

	// Get helpers settings from parsed layers
	helpersSettings := &cmdlayers.HelpersSettings{}
	err = geppettoParsedLayers.InitializeStruct(cmdlayers.GeppettoHelpersSlug, helpersSettings)
	if err != nil {
		return errors.Wrap(err, "failed to initialize helpers settings")
	}

	// Update step settings from parsed layers
	stepSettings, err := settings.NewStepSettings()
	if err != nil {
		return errors.Wrap(err, "failed to create step settings")
	}
	err = stepSettings.UpdateFromParsedLayers(geppettoParsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to update step settings from parsed layers")
	}

	// Build seed Turn from helpers settings (system prompt and optional user prompt)
	seed := &turns.Turn{}
	if sp := strings.TrimSpace(helpersSettings.System); sp != "" {
		turns.AppendBlock(seed, turns.NewSystemTextBlock(sp))
	}
	// If there's a message-file or append-message-file, they will be handled downstream; here we just respect system
	if up := strings.TrimSpace(""); up != "" {
		turns.AppendBlock(seed, turns.NewUserTextBlock(up))
	}

	// Enable server-side tools when requested: attach built-in web_search definition
	if s.ServerTools {
		if seed.Data == nil {
			seed.Data = map[string]any{}
		}
		seed.Data["responses_server_tools"] = []any{map[string]any{"type": "web_search"}}
	}

	// Let PinocchioCommand manage the EventRouter lifecycle and default printers
	// (avoids duplicate routers/handlers and blocking issues)

	// Run with options (Turn-first)
	updatedTurn, err := c.pinocchioCmd.RunWithOptions(ctx,
		run.WithStepSettings(stepSettings),
		run.WithWriter(w),
		run.WithRunMode(run.RunModeBlocking),
		run.WithUISettings(&run.UISettings{
			Output:       helpersSettings.Output,
			WithMetadata: helpersSettings.WithMetadata,
			FullOutput:   helpersSettings.FullOutput,
		}),
	)
	if err != nil {
		return err
	}

	fmt.Println("\n--------------------------------")
	fmt.Println()

	// Print the final Turn in a chat-like format
	turns.FprintTurn(w, updatedTurn)

	return nil
}

func main() {
	err := clay.InitViper("pinocchio", rootCmd)
	cobra.CheckErr(err)

	commands, err := pinocchio_cmds.LoadFromYAML(testYaml)
	cobra.CheckErr(err)

	// Register the command as a normal cobra command and let it parse its step settings by itself
	err = cli.AddCommandsToRootCommand(
		rootCmd, commands, nil,
		cli.WithCobraMiddlewaresFunc(layers2.GetCobraCommandGeppettoMiddlewares),
		cli.WithCobraShortHelpLayers(layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
	)
	cobra.CheckErr(err)

	// Add a clearer chat command wrapper with geppetto layers
	if len(commands) == 1 {
		cmd := commands[0].(*pinocchio_cmds.PinocchioCommand)
		chatCmd, err := NewChatCommand(cmd)
		cobra.CheckErr(err)
		command, err := cli.BuildCobraCommand(chatCmd)
		cobra.CheckErr(err)
		rootCmd.AddCommand(command)
	}

	cobra.CheckErr(rootCmd.Execute())
}
