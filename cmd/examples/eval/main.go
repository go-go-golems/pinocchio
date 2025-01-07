package main

import (
	"os"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/pinocchio/cmd/examples/eval/eval"
	"github.com/go-go-golems/pinocchio/cmd/examples/eval/serve"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/spf13/cobra"
)

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

	evalCmd, err := eval.NewEvalCommand()
	if err != nil {
		panic(err)
	}

	webViewCmd, err := serve.NewWebViewCommand()
	if err != nil {
		panic(err)
	}

	err = cli.AddCommandsToRootCommand(
		rootCmd,
		[]cmds.Command{evalCmd, webViewCmd},
		nil,
		cli.WithCobraMiddlewaresFunc(pinocchio_cmds.GetCobraCommandGeppettoMiddlewares),
		cli.WithCobraShortHelpLayers(layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
	)
	cobra.CheckErr(err)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
