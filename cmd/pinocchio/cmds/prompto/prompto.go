package prompto

import (
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/prompto/cmd/prompto/cmds"
	"github.com/spf13/cobra"
)

var promptoCmd = &cobra.Command{
	Use:   "prompto",
	Short: "prompto generates prompting context from a list of repositories",
	Long:  "prompto loads a list of repositories from a yaml config file and generates prompting context from them",
}

func InitPromptoCmd(helpSystem *help.HelpSystem) (*cobra.Command, error) {
	promptoCmd.AddCommand(cmds.NewGetCommand())
	promptoCmd.AddCommand(cmds.NewListCommand())
	promptoCmd.AddCommand(cmds.NewServeCommand())
	command, err := cmds.NewConfigGroupCommand(helpSystem)
	if err != nil {
		return nil, err
	}
	promptoCmd.AddCommand(command)

	return promptoCmd, nil
}
