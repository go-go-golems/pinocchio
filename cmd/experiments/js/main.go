package main

import (
	"os"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/js"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
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

			// Create the new RuntimeEngine
			engine := js.NewRuntimeEngine()
			defer engine.Close()

			log.Info().Msg("Starting RuntimeEngine")

			// Add setup functions
			engine.AddSetupFunction(js.SetupDoubleStep())
			engine.AddSetupFunction(js.SetupConversation())
			engine.AddSetupFunction(js.SetupEmbeddings(stepSettings))
			engine.AddSetupFunction(js.SetupChatStepFactory(stepSettings))
			engine.AddSetupFunction(js.SetupDoneCallback())

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

			log.Info().Msg("Script execution completed")
		},
	}

	err = parser.AddToCobraCommand(runCmd)
	cobra.CheckErr(err)

	rootCmd.AddCommand(runCmd)

	err = rootCmd.Execute()
	cobra.CheckErr(err)
}
