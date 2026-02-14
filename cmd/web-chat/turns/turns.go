package turns

import (
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/spf13/cobra"
)

var turnsCmd = &cobra.Command{
	Use:   "turns",
	Short: "Inspect and migrate persisted webchat turn data",
	Long:  "Utilities for legacy turn snapshots and normalized turns/blocks backfill.",
}

func AddToRootCommand(root *cobra.Command) {
	backfillCmd, err := NewTurnsBackfillCommand()
	cobra.CheckErr(err)

	cobraBackfillCmd, err := cli.BuildCobraCommand(backfillCmd)
	cobra.CheckErr(err)

	turnsCmd.AddCommand(cobraBackfillCmd)
	root.AddCommand(turnsCmd)
}
