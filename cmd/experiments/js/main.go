package main

import (
	"fmt"
	"os"

	"github.com/dop251/goja"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	pinocchio_docs "github.com/go-go-golems/pinocchio/cmd/pinocchio/doc"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
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
			vm := goja.New()
			setupConsole(vm)
			for _, scriptPath := range args {
				code, err := os.ReadFile(scriptPath)
				cobra.CheckErr(err)
				_, err = vm.RunString(string(code))
				cobra.CheckErr(err)
			}
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
