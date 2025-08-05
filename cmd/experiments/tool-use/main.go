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
	"github.com/go-go-golems/geppetto/pkg/toolbox"
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
	Use:   "tool-use",
	Short: "Tool use example with weather tool",
}

type ToolUseCommand struct {
	*cmds.CommandDescription
	pinocchioCmd *pinocchio_cmds.PinocchioCommand
}

type ToolUseSettings struct {
	PinocchioProfile string `glazed.parameter:"pinocchio-profile"`
	Debug            bool   `glazed.parameter:"debug"`
	CLIMode          bool   `glazed.parameter:"cli-mode"`
}

func NewToolUseCommand(cmd *pinocchio_cmds.PinocchioCommand) *ToolUseCommand {
	return &ToolUseCommand{
		CommandDescription: cmds.NewCommandDescription("tool-use",
			cmds.WithShort("Weather tool use example"),
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
			),
		),
		pinocchioCmd: cmd,
	}
}

func (c *ToolUseCommand) RunIntoWriter(ctx context.Context, parsedLayers *layers.ParsedLayers, w io.Writer) error {
	s := &ToolUseSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return errors.Wrap(err, "failed to initialize settings")
	}

	// Parse geppetto layers with the specified profile
	geppettoParsedLayers, err := helpers.ParseGeppettoLayers(c.pinocchioCmd, helpers.WithProfile(s.PinocchioProfile))
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

	// Create toolbox with weather tool
	tb := toolbox.NewRealToolbox()
	weatherTool := &WeatherTool{}
	err = tb.RegisterTool("get_weather", weatherTool.GetWeather)
	if err != nil {
		return errors.Wrap(err, "failed to register weather tool")
	}

	// Create base engine first 
	baseEngine, err := inference.NewEngineFromParsedLayers(geppettoParsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to create base engine")
	}

	// Create tool middleware
	toolConfig := inference.ToolConfig{
		MaxIterations: 5,
		Timeout:       30, // seconds
	}
	toolMiddleware := inference.NewToolMiddleware(tb, toolConfig)

	// Wrap engine with tool middleware
	engine := inference.NewEngineWithMiddleware(baseEngine, toolMiddleware)

	// Create image paths from helper settings
	imagePaths := make([]string, len(helpersSettings.Images))
	for i, img := range helpersSettings.Images {
		imagePaths[i] = img.Path
	}

	// Create the conversation manager
	b := c.pinocchioCmd.CreateConversationManagerBuilder()
	manager, err := b.WithImages(imagePaths).
		WithAutosaveSettings(builder.AutosaveSettings{
			Enabled:  strings.ToLower(helpersSettings.Autosave.Enabled) == "yes",
			Template: helpersSettings.Autosave.Template,
			Path:     helpersSettings.Autosave.Path,
		}).
		Build()
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
	fmt.Printf("Tool available: get_weather\n")
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
func (c *ToolUseCommand) runWithEngine(ctx context.Context, manager conversation.Manager, engine inference.Engine, runMode run.RunMode, uiSettings *run.UISettings, w io.Writer) ([]*conversation.Message, error) {
	// Get current conversation
	conversation_ := manager.GetConversation()

	// Run inference with our tool-enabled engine
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

	// Add the tool-use command as wrapped by NewToolUseCommand
	if len(commands) == 1 {
		cmd := commands[0].(*pinocchio_cmds.PinocchioCommand)
		toolCmd := NewToolUseCommand(cmd)
		command, err := cli.BuildCobraCommand(toolCmd)
		cobra.CheckErr(err)
		rootCmd.AddCommand(command)
	}

	cobra.CheckErr(rootCmd.Execute())
}
