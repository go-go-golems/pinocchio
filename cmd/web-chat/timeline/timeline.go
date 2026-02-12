package timeline

import (
	"github.com/go-go-golems/glazed/pkg/cli"
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
	statsCmd, err := NewTimelineStatsCommand()
	cobra.CheckErr(err)
	verifyCmd, err := NewTimelineVerifyCommand()
	cobra.CheckErr(err)

	cobraListCmd, err := cli.BuildCobraCommand(listCmd)
	cobra.CheckErr(err)
	cobraSnapshotCmd, err := cli.BuildCobraCommand(snapshotCmd)
	cobra.CheckErr(err)
	cobraEntitiesCmd, err := cli.BuildCobraCommand(entitiesCmd)
	cobra.CheckErr(err)
	cobraEntityCmd, err := cli.BuildCobraCommand(entityCmd)
	cobra.CheckErr(err)
	cobraStatsCmd, err := cli.BuildCobraCommand(statsCmd)
	cobra.CheckErr(err)
	cobraVerifyCmd, err := cli.BuildCobraCommand(verifyCmd)
	cobra.CheckErr(err)

	timelineCmd.AddCommand(cobraListCmd)
	timelineCmd.AddCommand(cobraSnapshotCmd)
	timelineCmd.AddCommand(cobraEntitiesCmd)
	timelineCmd.AddCommand(cobraEntityCmd)
	timelineCmd.AddCommand(cobraStatsCmd)
	timelineCmd.AddCommand(cobraVerifyCmd)

	root.AddCommand(timelineCmd)
}
