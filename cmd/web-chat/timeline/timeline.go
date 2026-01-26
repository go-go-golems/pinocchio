package timeline

import (
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/middlewares"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/spf13/cobra"
)

var timelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "Inspect persisted webchat timeline data",
	Long:  "Read-only CLI tools for inspecting timeline persistence and hydration snapshots.",
}

func AddToRootCommand(root *cobra.Command) {
	listCmd, err := NewTimelineListCommand()
	cobra.CheckErr(err)
	snapshotCmd, err := NewTimelineSnapshotCommand()
	cobra.CheckErr(err)
	entitiesCmd, err := NewTimelineEntitiesCommand()
	cobra.CheckErr(err)
	entityCmd, err := NewTimelineEntityCommand()
	cobra.CheckErr(err)

	cobraListCmd, err := cli.BuildCobraCommand(listCmd, cli.WithCobraMiddlewaresFunc(timelineMiddlewares))
	cobra.CheckErr(err)
	cobraSnapshotCmd, err := cli.BuildCobraCommand(snapshotCmd, cli.WithCobraMiddlewaresFunc(timelineMiddlewares))
	cobra.CheckErr(err)
	cobraEntitiesCmd, err := cli.BuildCobraCommand(entitiesCmd, cli.WithCobraMiddlewaresFunc(timelineMiddlewares))
	cobra.CheckErr(err)
	cobraEntityCmd, err := cli.BuildCobraCommand(entityCmd, cli.WithCobraMiddlewaresFunc(timelineMiddlewares))
	cobra.CheckErr(err)

	timelineCmd.AddCommand(cobraListCmd)
	timelineCmd.AddCommand(cobraSnapshotCmd)
	timelineCmd.AddCommand(cobraEntitiesCmd)
	timelineCmd.AddCommand(cobraEntityCmd)

	root.AddCommand(timelineCmd)
}

func timelineMiddlewares(
	_ *layers.ParsedLayers,
	cmd *cobra.Command,
	args []string,
) ([]middlewares.Middleware, error) {
	return []middlewares.Middleware{
		middlewares.ParseFromCobraCommand(cmd,
			parameters.WithParseStepSource("cobra"),
		),
		middlewares.GatherArguments(args,
			parameters.WithParseStepSource("arguments"),
		),
		middlewares.UpdateFromEnv("PINOCCHIO",
			parameters.WithParseStepSource("env"),
		),
		middlewares.SetFromDefaults(
			parameters.WithParseStepSource("defaults"),
		),
	}, nil
}
