package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/spf13/cobra"
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

	conversationContext, err := command.CreateConversationContext(nil)
	if err != nil {
		return fmt.Errorf("failed to create conversation context: %w", err)
	}

	manager := conversationContext.GetManager()
	conversation := manager.GetConversation()
	for _, msg := range conversation {
		fmt.Println(msg.Content.View())
	}

	// Process each entry in the dataset
	for i, entry := range dataset {
		// For now, just output the entry data as rows
		row := types.NewRow(
			types.MRP("entry_id", i+1),
			types.MRP("input", entry.Input),
			types.MRP("golden_answer", entry.GoldenAnswer),
		)

		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

func NewEvalCommand() (*EvalCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
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
		),
	}, nil
}

func main() {
	evalCmd, err := NewEvalCommand()
	if err != nil {
		panic(err)
	}

	rootCmd := &cobra.Command{
		Use:   "eval",
		Short: "Evaluate prompts against a dataset",
	}

	cobraCmd, err := cli.BuildCobraCommandFromGlazeCommand(evalCmd)
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cobraCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
