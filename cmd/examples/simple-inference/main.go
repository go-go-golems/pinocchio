package main

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/conversation/builder"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/inference"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/helpers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

//go:embed command.yaml
var commandYaml []byte

var rootCmd = &cobra.Command{
	Use:   "simple-inference",
	Short: "Simple inference example with Engine-first architecture",
}

type SimpleInferenceCommand struct {
	*cmds.CommandDescription
	pinocchioCmd *pinocchio_cmds.PinocchioCommand
}

type SimpleInferenceSettings struct {
	PinocchioProfile string `glazed.parameter:"pinocchio-profile"`
	Debug            bool   `glazed.parameter:"debug"`
	CLIMode          bool   `glazed.parameter:"cli-mode"`
	WithLogging      bool   `glazed.parameter:"with-logging"`
}

func NewSimpleInferenceCommand(cmd *pinocchio_cmds.PinocchioCommand) *SimpleInferenceCommand {
	return &SimpleInferenceCommand{
		CommandDescription: cmds.NewCommandDescription("simple-inference",
			cmds.WithShort("Simple inference with Engine-first architecture"),
			cmds.WithFlags(
				parameters.NewParameterDefinition("pinocchio-profile",
					parameters.ParameterTypeString,
					parameters.WithHelp("Pinocchio profile"),
					parameters.WithDefault("4o-mini"),
				),
				parameters.NewParameterDefinition("debug",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Debug mode - show parsed layers"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition("cli-mode",
					parameters.ParameterTypeBool,
					parameters.WithHelp("CLI mode - single inference without chat UI"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition("with-logging",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Enable logging middleware"),
					parameters.WithDefault(false),
				),
			),
		),
		pinocchioCmd: cmd,
	}
}

func (c *SimpleInferenceCommand) RunIntoWriter(ctx context.Context, parsedLayers *layers.ParsedLayers, w io.Writer) error {
	s := &SimpleInferenceSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return errors.Wrap(err, "failed to initialize settings")
	}

	// Parse geppetto layers with the specified profile
	// geppettoParsedLayers, err := helpers.ParseGeppettoLayers(c.pinocchioCmd, helpers.WithProfile(s.PinocchioProfile))
	geppettoParsedLayers, err := helpers.ParseGeppettoLayers(c.pinocchioCmd)
	if err != nil {
		return err
	}

	if s.Debug {
		// Marshal geppettoParsedLayer to yaml and print it
		b_, err := yaml.Marshal(geppettoParsedLayers)
		if err != nil {
			return err
		}
		fmt.Println("=== Parsed Layers Debug ===")
		fmt.Println(string(b_))
		fmt.Println("=========================")
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

	// Create base engine first
	baseEngine, err := inference.NewEngineFromParsedLayers(geppettoParsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to create base engine")
	}

	// Create middlewares
	var middlewares []inference.Middleware

	if s.WithLogging {
		// Add simple logging middleware
		loggingMiddleware := func(next inference.HandlerFunc) inference.HandlerFunc {
			return func(ctx context.Context, messages conversation.Conversation) (conversation.Conversation, error) {
				fmt.Fprintf(w, "[LOG] Starting inference with %d messages\n", len(messages))
				result, err := next(ctx, messages)
				if err != nil {
					fmt.Fprintf(w, "[LOG] Inference failed: %v\n", err)
				} else {
					fmt.Fprintf(w, "[LOG] Inference completed, %d messages in result\n", len(result))
				}
				return result, err
			}
		}
		middlewares = append(middlewares, loggingMiddleware)
	}

	// Wrap engine with middleware if any
	var engine inference.Engine
	if len(middlewares) > 0 {
		engine = inference.NewEngineWithMiddleware(baseEngine, middlewares...)
	} else {
		engine = baseEngine
	}

	// Create image paths from helper settings
	imagePaths := make([]string, len(helpersSettings.Images))
	for i, img := range helpersSettings.Images {
		imagePaths[i] = img.Path
	}

	// Create the conversation manager
	manager, err := c.pinocchioCmd.CreateConversationManager(
		geppettoParsedLayers.GetDefaultParameterLayer().Parameters.ToMap(),
		builder.WithImages(imagePaths),
		builder.WithAutosaveSettings(builder.AutosaveSettings{
			Enabled:  strings.ToLower(helpersSettings.Autosave.Enabled) == "yes",
			Template: helpersSettings.Autosave.Template,
			Path:     helpersSettings.Autosave.Path,
		}),
	)
	if err != nil {
		return err
	}

	// Determine run mode
	runMode := run.RunModeBlocking
	if s.CLIMode {
		runMode = run.RunModeBlocking
	} else if helpersSettings.StartInChat {
		runMode = run.RunModeChat
	} else if helpersSettings.Interactive {
		runMode = run.RunModeInteractive
	}

	fmt.Printf("Using profile: %s\n", s.PinocchioProfile)
	if stepSettings.Chat.Engine != nil {
		fmt.Printf("Model: %s\n", *stepSettings.Chat.Engine)
	}
	if s.WithLogging {
		fmt.Printf("Logging middleware: enabled\n")
	}
	fmt.Printf("Engine type: %T\n", engine)
	fmt.Println("---")

	// Run with engine override
	messages, err := c.runWithEngine(ctx, manager, engine, runMode, &run.UISettings{
		Output:       helpersSettings.Output,
		WithMetadata: helpersSettings.WithMetadata,
		FullOutput:   helpersSettings.FullOutput,
	}, w)
	if err != nil {
		return err
	}

	fmt.Println("\n=== Final Conversation ===")
	for _, msg := range messages {
		if chatMsg, ok := msg.Content.(*conversation.ChatMessageContent); ok {
			fmt.Printf("%s: %s\n", chatMsg.Role, chatMsg.Text)
		} else {
			fmt.Printf("%s: %s\n", msg.Content.ContentType(), msg.Content.String())
		}
	}

	return nil
}

// runWithEngine is a simplified version that uses our engine directly
func (c *SimpleInferenceCommand) runWithEngine(ctx context.Context, manager conversation.Manager, engine inference.Engine, runMode run.RunMode, uiSettings *run.UISettings, w io.Writer) ([]*conversation.Message, error) {
	// Get current conversation
	conversation_ := manager.GetConversation()

	// Run inference with our engine
	msg, err := engine.RunInference(ctx, conversation_)
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Append the result message to the conversation
	if err := manager.AppendMessages(msg); err != nil {
		return nil, fmt.Errorf("failed to append message: %w", err)
	}

	return manager.GetConversation(), nil
}

func main() {
	err := clay.InitViper("pinocchio", rootCmd)
	cobra.CheckErr(err)

	helpSystem := help.NewHelpSystem()
	help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

	commands, err := pinocchio_cmds.LoadFromYAML(commandYaml)
	cobra.CheckErr(err)

	// Register the command as a normal cobra command and let it parse its step settings by itself
	err = cli.AddCommandsToRootCommand(
		rootCmd, commands, nil,
		cli.WithCobraMiddlewaresFunc(pinocchio_cmds.GetCobraCommandGeppettoMiddlewares),
		cli.WithCobraShortHelpLayers(layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
	)
	cobra.CheckErr(err)

	// Add the simple-inference command as wrapped by NewSimpleInferenceCommand
	if len(commands) == 1 {
		cmd := commands[0].(*pinocchio_cmds.PinocchioCommand)
		simpleCmd := NewSimpleInferenceCommand(cmd)
		command, err := cli.BuildCobraCommand(simpleCmd)
		cobra.CheckErr(err)
		rootCmd.AddCommand(command)
	}

	cobra.CheckErr(rootCmd.Execute())
}
