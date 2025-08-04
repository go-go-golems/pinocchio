package main

import (
	"fmt"

	"github.com/go-go-golems/geppetto/pkg/inference"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
)

// ExampleWithParsedLayers demonstrates how to use NewEngineFromParsedLayers
// This would typically be used in a more complex command structure where
// parsedLayers are created by the command system during argument processing.
//
// NOTE: This is just a documentation example - in real usage, parsedLayers
// would be provided by the command system, not created manually like this.
func ExampleWithParsedLayersUsage() {
	// In a real pinocchio command, you would receive parsedLayers as a parameter
	// from the command system. Here's how you would use it:
	
	/*
	func (g *PinocchioCommand) RunIntoWriter(
		ctx context.Context,
		parsedLayers *layers.ParsedLayers,  // <- provided by command system
		w io.Writer,
	) error {
		// Create engine directly from parsed layers using the helper
		engine, err := inference.NewEngineFromParsedLayers(parsedLayers)
		if err != nil {
			return fmt.Errorf("failed to create engine from parsed layers: %w", err)
		}
		
		// Now use the engine...
		response, err := engine.RunInference(ctx, messages)
		// ...
	}
	*/
	
	// For demonstration purposes, create empty parsed layers
	parsedLayers := layers.NewParsedLayers()
	
	// This will create an engine with default settings since no layers are configured
	engine, err := inference.NewEngineFromParsedLayers(parsedLayers)
	if err != nil {
		fmt.Printf("Error creating engine: %v\n", err)
		return
	}
	
	fmt.Printf("Successfully created engine from parsed layers: %T\n", engine)
}
