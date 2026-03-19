package helpers

import (
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/spf13/cobra"
)

var helpersCmd = &cobra.Command{
	Use:   "helpers",
	Short: "Helper commands for common tasks",
	Long:  "A collection of helper commands for common tasks like markdown processing.",
}

func RegisterHelperCommands(rootCmd *cobra.Command) error {
	mdExtractCmd, err := NewExtractMdCommand()
	if err != nil {
		return err
	}

	mdExtractCobraCmd, err := cli.BuildCobraCommandFromCommand(mdExtractCmd)
	if err != nil {
		return err
	}

	helpersCmd.AddCommand(mdExtractCobraCmd)
	rootCmd.AddCommand(helpersCmd)
	return nil
}
