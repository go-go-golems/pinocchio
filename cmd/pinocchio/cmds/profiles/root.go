package profiles

import (
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/spf13/cobra"
)

func NewProfilesCommand() (*cobra.Command, error) {
	root := &cobra.Command{
		Use:   "profiles",
		Short: "Inspect Pinocchio engine profiles",
	}

	listCmd, err := NewListCommand()
	if err != nil {
		return nil, err
	}
	cobraListCmd, err := cli.BuildCobraCommand(listCmd)
	if err != nil {
		return nil, err
	}
	root.AddCommand(cobraListCmd)

	showCmd, err := NewShowCommand()
	if err != nil {
		return nil, err
	}
	cobraShowCmd, err := cli.BuildCobraCommand(showCmd)
	if err != nil {
		return nil, err
	}
	root.AddCommand(cobraShowCmd)

	return root, nil
}
