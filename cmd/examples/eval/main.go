package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	clay "github.com/go-go-golems/clay/pkg"
	pinocchio_settings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	glazed_settings "github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
)

type EvalCommand struct {
	*cmds.CommandDescription
}

type EvalSettings struct {
	Dataset string `glazed.parameter:"dataset"`
	Command string `glazed.parameter:"command"`
}

// Add these new structs for the eval dataset
type EvalEntry struct {
	Input        map[string]interface{} `json:"input"`
	GoldenAnswer interface{}            `json:"golden_answer"`
}

type EvalDataset []EvalEntry

func (c *EvalCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &EvalSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	// Read and parse the dataset file
	datasetBytes, err := os.ReadFile(s.Dataset)
	if err != nil {
		return fmt.Errorf("failed to read dataset file: %w", err)
	}

	var dataset EvalDataset
	if err := json.Unmarshal(datasetBytes, &dataset); err != nil {
		return fmt.Errorf("failed to parse dataset JSON: %w", err)
	}

	// Load the command from the YAML file
	commandBytes, err := os.ReadFile(s.Command)
	if err != nil {
		return fmt.Errorf("failed to read command file: %w", err)
	}

	commands, err := pinocchio_cmds.LoadFromYAML(commandBytes)
	if err != nil {
		return fmt.Errorf("failed to parse command YAML: %w", err)
	}

	if len(commands) != 1 {
		return fmt.Errorf("expected exactly one command in YAML, got %d", len(commands))
	}

	command, ok := commands[0].(*pinocchio_cmds.GeppettoCommand)
	if !ok {
		return fmt.Errorf("expected the command to be a GeppettoCommand, got %T", commands[0])
	}

	// Process each entry in the dataset
	for i, entry := range dataset {
		// Create a new conversation context for each entry
		conversationContext, err := command.CreateConversationContext(entry.Input)
		if err != nil {
			return fmt.Errorf("failed to create conversation context for entry %d: %w", i+1, err)
		}

		manager := conversationContext.GetManager()
		conversation := manager.GetConversation()

		// XXX CALL THE LLM

		// // Update step settings from parsed layers
		stepSettings := command.StepSettings.Clone()
		err = stepSettings.UpdateFromParsedLayers(parsedLayers)
		if err != nil {
			return err
		}

		// command.Run(stepSettings, conversation)

		// NOTE(manuel, 2025-01-04): not sure if we want to render the conversation inside Run or make it a parameter, maybe both.
		resultConversation, err := command.RunStepBlockingWithSettings(ctx, stepSettings, entry.Input)
		if err != nil {
			return fmt.Errorf("failed to run command for entry %d: %w", i+1, err)
		}

		// Get the AI's response
		var conversationString string
		for _, msg := range conversation {
			conversationString += msg.Content.View() + "\n"
		}

		// get the last message from the resultConversation
		lastMessage := resultConversation[len(resultConversation)-1]

		// Create a row with the entry data and AI response
		row := types.NewRow(
			types.MRP("entry_id", i+1),
			types.MRP("input", entry.Input),
			types.MRP("golden_answer", entry.GoldenAnswer),
			types.MRP("conversationString", conversationString),
			types.MRP("conversation", conversation),
			types.MRP("last_message", lastMessage.Content.View()),
			// XXX make sure the model and all that is in the message
			types.MRP("message_metadata", lastMessage.Metadata),
		)

		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}

	}

	return nil
}

func NewEvalCommand() (*EvalCommand, error) {
	glazedParameterLayer, err := glazed_settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, err
	}

	stepSettings, err := pinocchio_settings.NewStepSettings()
	if err != nil {
		return nil, err
	}

	chatParameterLayer, err := pinocchio_settings.NewChatParameterLayer(
		layers.WithDefaults(stepSettings.Chat),
	)
	if err != nil {
		return nil, err
	}

	clientParameterLayer, err := pinocchio_settings.NewClientParameterLayer(
		layers.WithDefaults(stepSettings.Client),
	)
	if err != nil {
		return nil, err
	}

	claudeParameterLayer, err := claude.NewParameterLayer(
		layers.WithDefaults(stepSettings.Claude),
	)
	if err != nil {
		return nil, err
	}
	openaiParameterLayer, err := openai.NewParameterLayer(
		layers.WithDefaults(stepSettings.OpenAI),
	)
	if err != nil {
		return nil, err
	}

	embeddingsParameterLayer, err := pinocchio_settings.NewEmbeddingsParameterLayer(
		layers.WithDefaults(stepSettings.Embeddings),
	)
	if err != nil {
		return nil, err
	}

	return &EvalCommand{
		CommandDescription: cmds.NewCommandDescription(
			"eval",
			cmds.WithShort("Evaluate prompts against a dataset"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"dataset",
					parameters.ParameterTypeString,
					parameters.WithHelp("Path to the eval dataset JSON file"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"command",
					parameters.ParameterTypeString,
					parameters.WithHelp("Path to the prompt template YAML file"),
					parameters.WithRequired(true),
				),
			),
			cmds.WithLayersList(glazedParameterLayer),
			cmds.WithLayersList(chatParameterLayer, clientParameterLayer, claudeParameterLayer, openaiParameterLayer, embeddingsParameterLayer),
		),
	}, nil
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "eval",
		Short: "Evaluate prompts against a dataset",
	}

	err := clay.InitViper("pinocchio", rootCmd)
	cobra.CheckErr(err)
	err = clay.InitLogger()
	cobra.CheckErr(err)

	helpSystem := help.NewHelpSystem()

	helpSystem.SetupCobraRootCommand(rootCmd)
	evalCmd, err := NewEvalCommand()
	if err != nil {
		panic(err)
	}

	cli.AddCommandsToRootCommand(
		rootCmd, []cmds.Command{evalCmd}, nil,
		cli.WithCobraMiddlewaresFunc(pinocchio_cmds.GetCobraCommandGeppettoMiddlewares),
		cli.WithCobraShortHelpLayers(layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
