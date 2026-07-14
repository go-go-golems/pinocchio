// Package auth provides Pinocchio's local OAuth browser-login commands.
package auth

import (
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/spf13/cobra"
)

// NewAuthCommand constructs the authentication command group. The Cobra group
// contains no flags; its user-facing verbs are Glazed commands.
func NewAuthCommand() (*cobra.Command, error) {
	root := &cobra.Command{
		Use:   "auth",
		Short: "Manage Pinocchio OAuth profile credentials",
	}
	login, err := NewLoginCommand()
	if err != nil {
		return nil, err
	}
	status, err := NewStatusCommand()
	if err != nil {
		return nil, err
	}
	logout, err := NewLogoutCommand()
	if err != nil {
		return nil, err
	}
	for _, command := range []cmds.GlazeCommand{login, status, logout} {
		cobraCommand, err := cli.BuildCobraCommand(command)
		if err != nil {
			return nil, err
		}
		root.AddCommand(cobraCommand)
	}
	return root, nil
}
