package catter

import (
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/pinocchio/cmd/pinocchio/cmds/catter/cmds"
	"github.com/spf13/cobra"
)

var catterCmd = &cobra.Command{
	Use:   "catter",
	Short: "Catter - File content and statistics tool",
	Long:  "A CLI tool to print file contents, recursively process directories, and count tokens for LLM context preparation.",
}

func AddToRootCommand(rootCmd *cobra.Command) {
	catterPrintCommand, err := cmds.NewCatterPrintCommand()
	cobra.CheckErr(err)

	catterStatsCmd, err := cmds.NewCatterStatsCommand()
	cobra.CheckErr(err)

	catterCobraCmd, err := cli.BuildCobraCommand(catterPrintCommand,
		cli.WithCobraMiddlewaresFunc(getMiddlewares),
	)
	cobra.CheckErr(err)

	catterStatsCobraCmd, err := cli.BuildCobraCommand(catterStatsCmd,
		cli.WithCobraMiddlewaresFunc(getMiddlewares),
	)
	cobra.CheckErr(err)

	catterCmd.AddCommand(catterCobraCmd)
	catterCmd.AddCommand(catterStatsCobraCmd)
	rootCmd.AddCommand(catterCmd)
}

func getMiddlewares(
	_ *values.Values,
	cmd *cobra.Command,
	args []string,
) ([]sources.Middleware, error) {
	return []sources.Middleware{
		sources.FromCobra(cmd),
		sources.FromArgs(args),
		sources.FromEnv("PINOCCHIO",
			fields.WithSource("env"),
		),
		sources.FromDefaults(),
	}, nil
}
