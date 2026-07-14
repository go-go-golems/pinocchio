package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	glazedsettings "github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

type StatusCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = (*StatusCommand)(nil)

func NewStatusCommand() (*StatusCommand, error) {
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
	return &StatusCommand{CommandDescription: cmds.NewCommandDescription(
		"status",
		cmds.WithShort("Show secret-free local OAuth credential readiness for a profile"),
		cmds.WithLong(`Show local OAuth credential readiness for the selected profile.

The command reads only the owner direct-YAML profile registry. It does not call a
provider, trigger refresh, or print access tokens, refresh tokens, expiry values,
client secrets, or registry paths.`),
		cmds.WithSections(glazedSection, commandSettingsSection, profileSettingsSection),
	)}, nil
}

func (c *StatusCommand) RunIntoGlazeProcessor(ctx context.Context, parsed *values.Values, gp middlewares.Processor) error {
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
	credential, err := oauthProfile.Store.Load(ctx, oauthProfile.Request)
	if err != nil {
		return fmt.Errorf("read local OAuth credential state: %w", err)
	}
	return gp.AddRow(ctx, types.NewRow(
		types.MRP("profile", profile),
		types.MRP("registry", registry),
		types.MRP("storage", "direct_yaml"),
		types.MRP("credential_state", credentialState(credential, time.Now())),
	))
}

func credentialState(credential credentials.Credential, now time.Time) string {
	// This classification deliberately returns readiness only. Callers must not
	// add credential fields or expiry values to command rows.
	if !credential.Usable(now, 0) {
		return "missing_or_expired"
	}
	if !credential.Usable(now, 5*time.Minute) {
		return "expiring"
	}
	return "usable"
}
