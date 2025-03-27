package main

import (
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/pinocchio/cmd/experiments/agent/codegen"
	"github.com/go-go-golems/pinocchio/cmd/experiments/agent/tool"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "agent",
	Short: "agent test",
}

func main() {
	helpSystem := help.NewHelpSystem()

	helpSystem.SetupCobraRootCommand(rootCmd)

	stepSettings, err := settings.NewStepSettings()
	cobra.CheckErr(err)

	geppettoLayers, err := cmds.CreateGeppettoLayers(stepSettings, cmds.WithHelpersLayer())
	cobra.CheckErr(err)

	pLayers := layers.NewParameterLayers(layers.WithLayers(geppettoLayers...))

	err = clay.InitViper("pinocchio", rootCmd)
	cobra.CheckErr(err)

	err = pLayers.AddToCobraCommand(upperCaseCmd)
	cobra.CheckErr(err)
	rootCmd.AddCommand(upperCaseCmd)

	err = pLayers.AddToCobraCommand(tool.ToolCallCmd)
	cobra.CheckErr(err)
	rootCmd.AddCommand(tool.ToolCallCmd)

	err = pLayers.AddToCobraCommand(codegen.CodegenTestCmd)
	cobra.CheckErr(err)
	rootCmd.AddCommand(codegen.CodegenTestCmd)

	err = pLayers.AddToCobraCommand(codegen.MultiStepCodgenTestCmd)
	cobra.CheckErr(err)
	rootCmd.AddCommand(codegen.MultiStepCodgenTestCmd)

	err = rootCmd.Execute()
	cobra.CheckErr(err)
}
