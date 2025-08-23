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
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/helpers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/go-go-golems/geppetto/pkg/turns"

	"github.com/go-go-golems/geppetto/pkg/events"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
)

//go:embed test.yaml
var testYaml []byte

var rootCmd = &cobra.Command{
	Use:   "simple-chat-step",
	Short: "A simple chat step",
}

type TestCommand struct {
	*cmds.CommandDescription
	pinocchioCmd *pinocchio_cmds.PinocchioCommand
}

type TestCommandSettings struct {
	PinocchioProfile string `glazed.parameter:"pinocchio-profile"`
	Debug            bool   `glazed.parameter:"debug"`
}

// NewTestCommand wraps the GepettoCommand which was loaded from the yaml file,
// and manually loads the profile to configure it.
func NewTestCommand(cmd *pinocchio_cmds.PinocchioCommand) *TestCommand {
	return &TestCommand{
		CommandDescription: cmds.NewCommandDescription("test2",
			cmds.WithShort("Test prompt"),
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
			),
		),
		pinocchioCmd: cmd,
	}
}

func (c *TestCommand) RunIntoWriter(ctx context.Context, parsedLayers *layers.ParsedLayers, w io.Writer) error {
	s := &TestCommandSettings{}
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

	// Build router using optional redis layer
	var router *events.EventRouter
	{
		// attempt to initialize from "redis" layer if present
		rs := rediscfg.Settings{}
		_ = geppettoParsedLayers.InitializeStruct("redis", &rs)
		r, err := rediscfg.BuildRouter(rs, false)
		if err != nil {
			return err
		}
		router = r
		defer func() { _ = router.Close() }()
		// default printer
		router.AddHandler("chat", "chat", events.StepPrinterFunc("", w))
	}

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
		run.WithRouter(router),
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

	// Add the test command as wrapped by NewTestCommand
	if len(commands) == 1 {
		cmd := commands[0].(*pinocchio_cmds.PinocchioCommand)
		testCmd := NewTestCommand(cmd)
		command, err := cli.BuildCobraCommand(testCmd)
		cobra.CheckErr(err)
		rootCmd.AddCommand(command)
	}

	cobra.CheckErr(rootCmd.Execute())
}
