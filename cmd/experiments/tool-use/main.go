package main

import (
    "context"
    _ "embed"
    "fmt"
    "io"
    "time"

    "github.com/go-go-golems/geppetto/pkg/conversation"
    
    "github.com/go-go-golems/geppetto/pkg/inference/engine"
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    "github.com/go-go-golems/geppetto/pkg/inference/toolhelpers"
    "github.com/go-go-golems/geppetto/pkg/inference/tools"
    layers2 "github.com/go-go-golems/geppetto/pkg/layers"

    clay "github.com/go-go-golems/clay/pkg"
    "github.com/go-go-golems/glazed/pkg/cli"
    "github.com/go-go-golems/glazed/pkg/cmds"
    "github.com/go-go-golems/glazed/pkg/cmds/layers"
    "github.com/go-go-golems/glazed/pkg/cmds/parameters"
    "github.com/go-go-golems/glazed/pkg/help"
    help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
    pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
    "github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
    "github.com/go-go-golems/pinocchio/pkg/cmds/helpers"
    "github.com/pkg/errors"
    "github.com/rs/zerolog/log"
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
)

//go:embed command.yaml
var commandYaml []byte

var rootCmd = &cobra.Command{
    Use:   "tool-use",
    Short: "Tool use example with weather tool (updated API)",
}

// WeatherRequest represents the JSON input for the weather tool
// This is simpler than the original GetWeather(city string) signature
// and works well with the reflection-based Tool registry.
type WeatherRequest struct {
    City string `json:"city" jsonschema:"required,description=City name to get weather for"`
}

// weatherTool is an adapter that calls the old WeatherTool.GetWeather method
func weatherTool(req WeatherRequest) (*WeatherResult, error) {
    wt := &WeatherTool{}
    return wt.GetWeather(req.City)
}

// ToolUseCommand wraps the generated pinocchio command with additional flags
// and demonstrates tool calling using the new inference helper APIs.
type ToolUseCommand struct {
    *cmds.CommandDescription
    pinocchioCmd *pinocchio_cmds.PinocchioCommand
}

type ToolUseSettings struct {
    PinocchioProfile string `glazed.parameter:"pinocchio-profile"`
    Debug            bool   `glazed.parameter:"debug"`
    Prompt           string `glazed.parameter:"prompt"`
}

func NewToolUseCommand(cmd *pinocchio_cmds.PinocchioCommand) *ToolUseCommand {
    return &ToolUseCommand{
        CommandDescription: cmds.NewCommandDescription("tool-use",
            cmds.WithShort("Weather tool use example (new inference API)"),
            cmds.WithArguments(
                parameters.NewParameterDefinition("prompt",
                    parameters.ParameterTypeString,
                    parameters.WithHelp("Prompt to run"),
                    parameters.WithRequired(true),
                ),
            ),
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
            ),
        ),
        pinocchioCmd: cmd,
    }
}

// RunIntoWriter implements the new helper-based tool calling workflow.
func (c *ToolUseCommand) RunIntoWriter(ctx context.Context, parsedLayers *layers.ParsedLayers, w io.Writer) error {
    s := &ToolUseSettings{}
    if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
        return errors.Wrap(err, "failed to initialize settings")
    }

    // 1. Parse Geppetto layers (provider configuration) with the selected profile.
    geppettoParsedLayers, err := helpers.ParseGeppettoLayers(c.pinocchioCmd, helpers.WithProfile(s.PinocchioProfile))
    if err != nil {
        return err
    }

    // Debug output of parsed layers if requested.
    if s.Debug {
        b_, _ := yaml.Marshal(geppettoParsedLayers)
        fmt.Fprintln(w, "=== Parsed Layers Debug ===")
        fmt.Fprintln(w, string(b_))
        fmt.Fprintln(w, "===========================")
        return nil
    }

    // 2. Build the base Engine from layers (provider-agnostic)
    baseEngine, err := factory.NewEngineFromParsedLayers(geppettoParsedLayers)
    if err != nil {
        return errors.Wrap(err, "failed to create engine")
    }

    // 3. Create Tool registry and register weather tool
    registry := tools.NewInMemoryToolRegistry()
    weatherToolDef, err := tools.NewToolFromFunc(
        "get_weather",
        "Get current weather information for a specific city",
        weatherTool,
    )
    if err != nil {
        return errors.Wrap(err, "failed to create weather tool definition")
    }
    if err := registry.RegisterTool("get_weather", *weatherToolDef); err != nil {
        return errors.Wrap(err, "failed to register weather tool")
    }

    // 4. If the underlying Engine supports tool configuration, provide the tool
    if configurableEngine, ok := baseEngine.(interface {
        ConfigureTools([]engine.ToolDefinition, engine.ToolConfig)
    }); ok {
        var engineTools []engine.ToolDefinition
        for _, t := range registry.ListTools() {
            engineTools = append(engineTools, engine.ToolDefinition{
                Name:        t.Name,
                Description: t.Description,
                Parameters:  t.Parameters,
            })
        }

        engineConfig := engine.ToolConfig{
            Enabled:           true,
            ToolChoice:        engine.ToolChoiceAuto,
            MaxIterations:     1, // Single iteration – orchestration handled by helpers
            ExecutionTimeout:  30 * time.Second,
            MaxParallelTools:  1,
            ToolErrorHandling: engine.ToolErrorContinue,
        }
        configurableEngine.ConfigureTools(engineTools, engineConfig)
    }

    // 5. Build conversation manager
    managerBuilder := c.pinocchioCmd.CreateConversationManagerBuilder()
    manager, err := managerBuilder.
        WithPrompt(s.Prompt).
        Build()
    if err != nil {
        return err
    }

    conversation_ := manager.GetConversation()

    // 6. Configure helper loop
    helperConfig := toolhelpers.NewToolConfig().
        WithMaxIterations(5).
        WithTimeout(30 * time.Second).
        WithMaxParallelTools(1).
        WithToolChoice(tools.ToolChoiceAuto).
        WithToolErrorHandling(tools.ToolErrorContinue)

    log.Info().Msg("Running inference with helper-based tool calling loop")

    updatedConversation, err := toolhelpers.RunToolCallingLoop(ctx, baseEngine, conversation_, registry, helperConfig)
    if err != nil {
        return errors.Wrap(err, "inference failed")
    }

    // Append new messages to manager for display
    newMessages := updatedConversation[len(conversation_):]
    if err := manager.AppendMessages(newMessages...); err != nil {
        return errors.Wrap(err, "failed to append messages")
    }

    fmt.Fprintln(w, "\n=== Final Conversation ===")
    for _, msg := range manager.GetConversation() {
        switch c := msg.Content.(type) {
        case *conversation.ChatMessageContent:
            fmt.Fprintf(w, "%s: %s\n", c.Role, c.Text)
        case *conversation.ToolUseContent:
            fmt.Fprintf(w, "Tool Call (%s): %s\n", c.Name, string(c.Input))
        case *conversation.ToolResultContent:
            fmt.Fprintf(w, "Tool Result (%s): %s\n", c.ToolID, c.Result)
        default:
            fmt.Fprintf(w, "%s: %s\n", msg.Content.ContentType(), msg.Content.String())
        }
    }

    return nil
}

func main() {
    // Standard pinocchio / glazed boilerplate
    if err := clay.InitViper("pinocchio", rootCmd); err != nil {
        panic(err)
    }

    helpSystem := help.NewHelpSystem()
    help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

    // Load the generated command (weather-chat) from YAML
    commands, err := pinocchio_cmds.LoadFromYAML(commandYaml)
    cobra.CheckErr(err)

    // Register the generated command(s) so that layers get parsed correctly
    err = cli.AddCommandsToRootCommand(
        rootCmd, commands, nil,
        cli.WithCobraMiddlewaresFunc(layers2.GetCobraCommandGeppettoMiddlewares),
        cli.WithCobraShortHelpLayers(layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
    )
    cobra.CheckErr(err)

    // Wrap with the updated ToolUseCommand that demonstrates the new API
    if len(commands) == 1 {
        cmd := commands[0].(*pinocchio_cmds.PinocchioCommand)
        toolCmd := NewToolUseCommand(cmd)
        cobraCmd, err := cli.BuildCobraCommand(toolCmd)
        cobra.CheckErr(err)
        rootCmd.AddCommand(cobraCmd)
    }

    cobra.CheckErr(rootCmd.Execute())
}
