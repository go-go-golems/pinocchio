package main

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
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
		return logging.InitLoggerFromCobra(cmd)
	},
}

type TestCommand struct {
	*cmds.CommandDescription
	pinocchioCmd *pinocchio_cmds.PinocchioCommand
}

type ChatCommandSettings struct {
	Debug       bool `glazed:"debug"`
	ServerTools bool `glazed:"server-tools"`
}

// NewChatCommand wraps the GepettoCommand which was loaded from the yaml file,
// and manually loads the profile to configure it.
func NewChatCommand(cmd *pinocchio_cmds.PinocchioCommand) (*TestCommand, error) {
	profileSettingsSection, err := helpers.NewProfileSettingsSection()
	if err != nil {
		return nil, err
	}
	return &TestCommand{
		CommandDescription: cmds.NewCommandDescription("chat",
			cmds.WithShort("Run chat with simple streaming printer"),
			cmds.WithFlags(
				fields.New("debug",
					fields.TypeBool,
					fields.WithHelp("Debug mode"),
					fields.WithDefault(false),
				),
				fields.New("server-tools",
					fields.TypeBool,
					fields.WithHelp("Enable Responses server-side tools (web_search)"),
					fields.WithDefault(false),
				),
			),
			cmds.WithSections(profileSettingsSection),
		),
		pinocchioCmd: cmd,
	}, nil
}

func (c *TestCommand) RunIntoWriter(ctx context.Context, parsedLayers *values.Values, w io.Writer) error {
	s := &ChatCommandSettings{}
	err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s)
	if err != nil {
		return errors.Wrap(err, "failed to initialize settings")
	}
	commandSettings := &cli.CommandSettings{}
	_ = parsedLayers.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings)
	profileSettings := helpers.ResolveProfileSettings(parsedLayers)
	if profileSettings.Profile == "" {
		profileSettings.Profile = "default"
	}

	geppettoParsedLayers, err := helpers.ParseGeppettoLayers(
		c.pinocchioCmd,
		helpers.WithProfile(profileSettings.Profile),
		helpers.WithProfileRegistries(profileSettings.ProfileRegistries),
		helpers.WithConfigFile(commandSettings.ConfigFile),
	)
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
	err = geppettoParsedLayers.DecodeSectionInto(cmdlayers.GeppettoHelpersSlug, helpersSettings)
	if err != nil {
		return errors.Wrap(err, "failed to initialize helpers settings")
	}

	// Update inference settings from parsed layers
	stepSettings, err := settings.NewInferenceSettings()
	if err != nil {
		return errors.Wrap(err, "failed to create inference settings")
	}
	err = stepSettings.UpdateFromParsedValues(geppettoParsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to update inference settings from parsed layers")
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
		if err := turns.KeyResponsesServerTools.Set(&seed.Data, []any{map[string]any{"type": "web_search"}}); err != nil {
			return errors.Wrap(err, "set responses server tools")
		}
	}

	// Let PinocchioCommand manage the EventRouter lifecycle and default printers
	// (avoids duplicate routers/handlers and blocking issues)

	// Run with options (Turn-first)
	updatedTurn, err := c.pinocchioCmd.RunWithOptions(ctx,
		run.WithInferenceSettings(stepSettings),
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
	err := clay.InitGlazed("pinocchio", rootCmd)
	cobra.CheckErr(err)

	commands, err := pinocchio_cmds.LoadFromYAML(testYaml)
	cobra.CheckErr(err)

	// Add a clearer chat command wrapper with geppetto layers
	if len(commands) == 1 {
		cmd := commands[0].(*pinocchio_cmds.PinocchioCommand)
		chatCmd, err := NewChatCommand(cmd)
		cobra.CheckErr(err)
		command, err := cli.BuildCobraCommand(chatCmd, cli.WithParserConfig(cli.CobraParserConfig{
			AppName: "pinocchio",
			ConfigFilesFunc: func(_ *values.Values, _ *cobra.Command, _ []string) ([]string, error) {
				return nil, nil
			},
		}))
		cobra.CheckErr(err)
		for _, name := range []string{"print-yaml", "print-parsed-fields", "print-schema"} {
			if flag := command.Flags().Lookup(name); flag != nil {
				flag.Hidden = true
			}
		}
		rootCmd.AddCommand(command)
	}

	cobra.CheckErr(rootCmd.Execute())
}
