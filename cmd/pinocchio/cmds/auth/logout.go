package auth

import (
	"context"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	glazedsettings "github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

type LogoutCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = (*LogoutCommand)(nil)

func NewLogoutCommand() (*LogoutCommand, error) {
	glazedSection, err := glazedsettings.NewGlazedSection()
	if err != nil {
		return nil, err
	}
	commandSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}
	profileSettingsSection, err := profilebootstrap.NewProfileSettingsSection()
	if err != nil {
		return nil, err
	}
	return &LogoutCommand{CommandDescription: cmds.NewCommandDescription(
		"logout",
		cmds.WithShort("Remove locally stored OAuth credentials for a profile"),
		cmds.WithLong(`Remove only the local OAuth credential tuple for the selected profile.

The command retains the profile's authorization URL, token URL, client ID, scopes,
and refresh policy so a later auth login can reuse the same profile. It does not
call a provider revocation endpoint or print credential material.`),
		cmds.WithSections(glazedSection, commandSettingsSection, profileSettingsSection),
	)}, nil
}

func (c *LogoutCommand) RunIntoGlazeProcessor(ctx context.Context, parsed *values.Values, gp middlewares.Processor) error {
	resolved, oauthProfile, err := resolveOAuthProfileForCommand(ctx, parsed)
	if err != nil {
		return err
	}
	defer func() {
		if resolved.Close != nil {
			resolved.Close()
		}
	}()
	profile, registry, err := profileAndRegistryRow(resolved)
	if err != nil {
		return err
	}
	if err := oauthProfile.Store.Delete(ctx, oauthProfile.Request); err != nil {
		return err
	}
	return gp.AddRow(ctx, types.NewRow(
		types.MRP("profile", profile),
		types.MRP("registry", registry),
		types.MRP("status", "logged_out"),
	))
}
