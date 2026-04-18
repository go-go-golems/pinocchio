package main

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"strings"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
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

func resolveCommandLayers(cmd *pinocchio_cmds.PinocchioCommand, parsed *values.Values) (*values.Values, error) {
	configFiles, err := profilebootstrap.ResolveCLIConfigFilesResolved(parsed)
	if err != nil {
		return nil, err
	}

	resolved := values.New()
	if err := sources.Execute(
		cmd.Description().Schema,
		resolved,
		sources.FromEnv("PINOCCHIO", fields.WithSource("env")),
		sources.FromResolvedFiles(
			configFiles.Files,
			sources.WithConfigFileMapper(profilebootstrap.MapPinocchioConfigFile),
			sources.WithParseOptions(fields.WithSource("config")),
		),
		sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	); err != nil {
		return nil, err
	}
	if parsed != nil {
		if err := resolved.Merge(parsed); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

// NewChatCommand wraps the GepettoCommand which was loaded from the yaml file,
// and manually loads the profile to configure it.
func NewChatCommand(cmd *pinocchio_cmds.PinocchioCommand) (*TestCommand, error) {
	profileSettingsSection, err := profilebootstrap.NewProfileSettingsSection()
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

	resolvedLayers, err := resolveCommandLayers(c.pinocchioCmd, parsedLayers)
	if err != nil {
		return err
	}

	if s.Debug {
		b_, err := yaml.Marshal(resolvedLayers)
		if err != nil {
			return err
		}
		fmt.Println(string(b_))
		return nil
	}

	helpersSettings := &cmdlayers.HelpersSettings{}
	err = resolvedLayers.DecodeSectionInto(cmdlayers.GeppettoHelpersSlug, helpersSettings)
	if err != nil {
		return errors.Wrap(err, "failed to initialize helpers settings")
	}

	resolvedSettings, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to resolve inference settings")
	}
	if resolvedSettings.Close != nil {
		defer resolvedSettings.Close()
	}
	stepSettings := resolvedSettings.FinalInferenceSettings
	if stepSettings == nil {
		return errors.New("resolved inference settings are nil")
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
