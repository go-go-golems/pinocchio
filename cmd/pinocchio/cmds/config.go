package cmds

import (
	"github.com/go-go-golems/clay/pkg/cmds/repositories"
	"github.com/go-go-golems/glazed/pkg/config"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/spf13/cobra"
)

// commands for manipulating the config
//
// - x - add/rm/print a repository entry
// - set ai keys
// - add profile / remove profile / update profile
//
// layers that are loaded from the config file:
// (from cobra.go)
// - ai-chat
// - ai-client
// - openai-chat
// - claude-chat

// NewConfigGroupCommand creates a new config command group for pinocchio
func NewConfigGroupCommand(helpSystem *help.HelpSystem) (*cobra.Command, error) {
	configCmd, err := config.NewConfigCommand("pinocchio")
	if err != nil {
		return nil, err
	}

	configCmd.AddCommand(repositories.NewRepositoriesGroupCommand())
	err = repositories.AddDocToHelpSystem(helpSystem)
	if err != nil {
		return nil, err
	}

	return configCmd.Command, nil
}
