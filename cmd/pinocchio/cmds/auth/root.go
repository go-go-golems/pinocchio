// Package auth provides Pinocchio's local OAuth browser-login commands.
package auth

import (
	"github.com/go-go-golems/glazed/pkg/cli"
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
	cobraLogin, err := cli.BuildCobraCommand(login)
	if err != nil {
		return nil, err
	}
	root.AddCommand(cobraLogin)
	return root, nil
}
