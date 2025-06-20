package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/helpers"
	"github.com/go-go-golems/geppetto/pkg/js"
	"github.com/go-go-golems/geppetto/pkg/steps"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	pinocchio_docs "github.com/go-go-golems/pinocchio/cmd/pinocchio/doc"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/dop251/goja"
)

var rootCmd = &cobra.Command{
	Use:   "js-experiments",
	Short: "JavaScript experiments for Geppetto",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logging.InitLoggerFromViper()
	},
}

var runCmd *cobra.Command

// TestDoubleStep is a test step that publishes events during execution
type TestDoubleStep struct {
	publisherManager *events.PublisherManager
}

func (t *TestDoubleStep) Start(ctx context.Context, input float64) (steps.StepResult[float64], error) {
	log.Debug().Float64("input", input).Msg("Starting TestDoubleStep execution")

	// Create result channel
	c := make(chan helpers.Result[float64], 1)

	go func() {
		defer close(c)

		// Publish start event
		if t.publisherManager != nil {
			startEvent := events.NewStartEvent(events.EventMetadata{}, &steps.StepMetadata{})
			t.publishEvent(startEvent)
		}

		// Simulate some work with delay
		fmt.Println("Starting doubleStep")
		time.Sleep(500 * time.Millisecond)

		result := input * 2

		// Publish final event
		if t.publisherManager != nil {
			finalEvent := events.NewFinalEvent(events.EventMetadata{}, &steps.StepMetadata{}, fmt.Sprintf("%.2f", result))
			t.publishEvent(finalEvent)
		}

		fmt.Println("Finished doubleStep")
		log.Debug().Float64("result", result).Msg("Completed TestDoubleStep execution")

		// Send final result to channel
		c <- helpers.NewValueResult(result)
	}()

	return steps.NewStepResult[float64](c), nil
}

func (t *TestDoubleStep) AddPublishedTopic(publisher message.Publisher, topic string) error {
	if t.publisherManager == nil {
		t.publisherManager = events.NewPublisherManager()
	}
	t.publisherManager.RegisterPublisher(topic, publisher)
	return nil
}

func (t *TestDoubleStep) publishEvent(event events.Event) {
	if t.publisherManager != nil {
		log.Debug().Str("eventType", string(event.Type())).Msg("Publishing event")
		err := t.publisherManager.Publish(event)
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish event")
		}
	}
}

// setupDoubleStep creates a setup function for a simple test step that doubles numbers
func setupDoubleStep() js.SetupFunction {
	return func(vm *goja.Runtime, engine *js.RuntimeEngine) {
		log.Debug().Msg("Setting up doubleStep")

		// Create a step that publishes events
		doubleStep := &TestDoubleStep{}
		log.Debug().Msg("Created testDoubleStep")

		// Create watermill-based step object factory
		log.Debug().Msg("Creating watermill step object factory")
		stepObjectFactory := js.CreateWatermillStepObject(
			engine,
			doubleStep,
			func(v goja.Value) float64 {
				val := v.ToFloat()
				log.Debug().Float64("value", val).Msg("Input converter called")
				return val
			},
			func(v float64) goja.Value {
				log.Debug().Float64("value", v).Msg("Output converter called")
				return vm.ToValue(v)
			},
		)

		// Create the step object in the VM context
		log.Debug().Msg("Creating step object in VM context")
		stepObj := stepObjectFactory(vm)

		log.Debug().Msg("Registering doubleStep in VM")
		err := vm.Set("doubleStep", stepObj)
		if err != nil {
			log.Error().Err(err).Msg("Failed to register doubleStep")
			return
		}
		log.Debug().Msg("doubleStep registered successfully")
	}
}

// setupDoneCallback creates a setup function that registers a done() callback
func setupDoneCallback() js.SetupFunction {
	return func(vm *goja.Runtime, engine *js.RuntimeEngine) {
		log.Debug().Msg("Setting up done callback")
		err := vm.Set("done", func(args ...interface{}) {
			log.Info().Msg("Done callback called")
			// Signal the engine to stop
			engine.Stop()
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to register done callback")
		}
	}
}

func main() {
	helpSystem := help.NewHelpSystem()

	err := pinocchio_docs.AddDocToHelpSystem(helpSystem)
	cobra.CheckErr(err)

	helpSystem.SetupCobraRootCommand(rootCmd)

	err = clay.InitViper("pinocchio", rootCmd)
	cobra.CheckErr(err)

	stepSettings, err := settings.NewStepSettings()
	cobra.CheckErr(err)
	geppettoLayers, err := cmds.CreateGeppettoLayers(stepSettings, cmds.WithHelpersLayer())
	cobra.CheckErr(err)

	// Add profile settings layer for --profile and --profile-file flags
	profileLayer, err := cli.NewProfileSettingsLayer()
	cobra.CheckErr(err)
	geppettoLayers = append(geppettoLayers, profileLayer)

	layers_ := layers.NewParameterLayers(layers.WithLayers(geppettoLayers...))

	parser, err := cli.NewCobraParserFromLayers(
		layers_,
		cli.WithCobraMiddlewaresFunc(cmds.GetCobraCommandGeppettoMiddlewares),
		cli.WithCobraShortHelpLayers(
			layers.DefaultSlug,
			cli.CommandSettingsSlug,
			cli.ProfileSettingsSlug,
		),
	)
	cobra.CheckErr(err)

	// Add run command that executes JS tests
	runCmd = &cobra.Command{
		Use:   "run [scripts...]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Run JavaScript scripts",
		Run: func(cmd *cobra.Command, args []string) {
			parsedLayers, err := parser.Parse(cmd, nil)
			cobra.CheckErr(err)

			err = stepSettings.UpdateFromParsedLayers(parsedLayers)
			cobra.CheckErr(err)

			// Create the new RuntimeEngine with setup functions
			engine, err := js.NewRuntimeEngine(
				js.WithSetupFunctions(
					setupDoubleStep(),
					js.SetupConversation(),
					js.SetupEmbeddings(stepSettings),
					js.SetupChatStepFactory(stepSettings),
					setupDoneCallback(),
				),
			)
			cobra.CheckErr(err)
			defer engine.Close()

			log.Info().Msg("Starting RuntimeEngine")

			// Read all scripts
			var allCode string
			for _, scriptPath := range args {
				log.Info().Str("script", scriptPath).Msg("Reading script")
				code, err := os.ReadFile(scriptPath)
				if err != nil {
					log.Error().Err(err).Msg("Failed to read script")
					os.Exit(1)
				}
				allCode += string(code) + "\n"
			}

			// Start the engine (this will block until completion)
			log.Debug().Msg("Starting RuntimeEngine")
			engine.Start()

			log.Debug().Msg("Running JavaScript code")

			engine.RunOnLoop(allCode)

			log.Info().Msg("Script execution completed - waiting for async operations")
			
			// Wait for the event loop to finish (it will be stopped by done() callback)
			time.Sleep(10 * time.Second) // Give enough time for async operations
		},
	}

	err = parser.AddToCobraCommand(runCmd)
	cobra.CheckErr(err)

	rootCmd.AddCommand(runCmd)

	err = rootCmd.Execute()
	cobra.CheckErr(err)
}
