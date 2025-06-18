package main

import (
	"fmt"
	"os"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/embeddings"
	"github.com/go-go-golems/geppetto/pkg/helpers"
	"github.com/go-go-golems/geppetto/pkg/js"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/utils"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	pinocchio_docs "github.com/go-go-golems/pinocchio/cmd/pinocchio/doc"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "js-experiments",
	Short: "JavaScript experiments for Geppetto",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logging.InitLoggerFromViper()
	},
}

var runCmd *cobra.Command

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

			// Create event loop
			loop := eventloop.NewEventLoop()
			loop.Start()
			defer loop.Stop()

			log.Info().Msg("Starting event loop")

			// Channel to wait for completion
			done := make(chan error, 1)

			loop.RunOnLoop(func(vm *goja.Runtime) {
				setupConsole(vm)
				setupJSEnvironment(vm, loop, stepSettings)

				// Register done callback
				doneCallbackUsed := false
				err := vm.Set("done", func(args ...interface{}) {
					doneCallbackUsed = true
					if len(args) > 0 {
						if err, ok := args[0].(error); ok {
							done <- err
						} else {
							done <- fmt.Errorf("script error: %v", args[0])
						}
					} else {
						done <- nil
					}
				})
				cobra.CheckErr(err)

				// Execute scripts
				for _, scriptPath := range args {
					log.Info().Str("script", scriptPath).Msg("Executing script")
					code, err := os.ReadFile(scriptPath)
					if err != nil {
						done <- err
						return
					}
					_, err = vm.RunString(string(code))
					if err != nil {
						done <- err
						return
					}
				}

				// If no async operations are needed, signal completion after a delay
				// This allows the script to call done() if needed
				go func() {
					time.Sleep(2 * time.Second) // Give more time for async operations
					if !doneCallbackUsed {
						select {
						case done <- nil:
						default:
						}
					}
				}()
			})

			// Wait for completion
			if err := <-done; err != nil {
				log.Error().Err(err).Msg("Script execution failed")
				os.Exit(1)
			}

			log.Info().Msg("Script execution completed")
		},
	}

	err = parser.AddToCobraCommand(runCmd)
	cobra.CheckErr(err)

	rootCmd.AddCommand(runCmd)

	err = rootCmd.Execute()
	cobra.CheckErr(err)
}

func setupConsole(vm *goja.Runtime) {
	console := vm.NewObject()
	_ = console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		fmt.Println(args...)
		return goja.Undefined()
	})
	_ = console.Set("error", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		fmt.Printf("ERROR: %v\n", args...)
		return goja.Undefined()
	})
	_ = vm.Set("console", console)
}

func setupJSEnvironment(vm *goja.Runtime, loop *eventloop.EventLoop, stepSettings *settings.StepSettings) {
	// Register step for stepTest.js
	setupDoubleStep(vm, loop)
	
	// Register conversation for conversationTest.js
	err := js.RegisterConversation(vm)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register conversation")
	}
	
	// Register chat step factory for chatStepTest.js
	err = js.RegisterFactory(vm, loop, stepSettings)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register factory")
	}
	
	// Register embeddings for embeddingsTest.js
	setupEmbeddings(vm, loop, stepSettings)
}

func setupDoubleStep(vm *goja.Runtime, loop *eventloop.EventLoop) {
	// Create a simple test step that doubles numbers with delay
	doubleStep := &utils.LambdaStep[float64, float64]{
		Function: func(input float64) helpers.Result[float64] {
			fmt.Println("Starting doubleStep")
			time.Sleep(500 * time.Millisecond)
			fmt.Println("Finished doubleStep")
			return helpers.NewValueResult(input * 2)
		},
	}

	// Register step in JS
	err := js.RegisterStep(
		vm,
		loop,
		"doubleStep",
		doubleStep,
		func(v goja.Value) float64 { return v.ToFloat() },
		func(v float64) goja.Value { return vm.ToValue(v) },
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register doubleStep")
	}
}

func setupEmbeddings(vm *goja.Runtime, loop *eventloop.EventLoop, stepSettings *settings.StepSettings) {
	factory := embeddings.NewSettingsFactoryFromStepSettings(stepSettings)
	provider, err := factory.NewProvider()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create embeddings provider")
		return
	}

	// Register embeddings in JavaScript
	err = js.RegisterEmbeddings(vm, "embeddings", provider, loop)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register embeddings")
	}
}
