package auth

import (
	"context"
	"fmt"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

func resolveOAuthProfileForCommand(ctx context.Context, parsed *values.Values) (*profilebootstrap.ResolvedCLIEngineSettings, *profilebootstrap.ResolvedOAuthProfile, error) {
	commandSettings := &cli.CommandSettings{}
	profileSettings := &profilebootstrap.ProfileSettings{}
	if parsed != nil {
		if err := parsed.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings); err != nil {
			return nil, nil, fmt.Errorf("decode auth command settings: %w", err)
		}
		if err := parsed.DecodeSectionInto(profilebootstrap.ProfileSettingsSectionSlug, profileSettings); err != nil {
			return nil, nil, fmt.Errorf("decode auth profile settings: %w", err)
		}
	}
	selection, err := profilebootstrap.NewCLISelectionValues(profilebootstrap.CLISelectionInput{
		ConfigFile:        commandSettings.ConfigFile,
		Profile:           profileSettings.Profile,
		ProfileRegistries: profileSettings.ProfileRegistries,
	})
	if err != nil {
		return nil, nil, err
	}
	resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, selection)
	if err != nil {
		return nil, nil, err
	}
	oauthProfile, err := profilebootstrap.ResolveOAuthProfile(ctx, resolved)
	if err != nil {
		if resolved.Close != nil {
			resolved.Close()
		}
		return nil, nil, err
	}
	if oauthProfile == nil {
		if resolved.Close != nil {
			resolved.Close()
		}
		return nil, nil, fmt.Errorf("selected profile is not an OAuth profile")
	}
	return resolved, oauthProfile, nil
}

func profileAndRegistryRow(resolved *profilebootstrap.ResolvedCLIEngineSettings) (string, string, error) {
	if resolved == nil || resolved.ResolvedEngineProfile == nil {
		return "", "", fmt.Errorf("selected OAuth profile is unavailable")
	}
	return resolved.ResolvedEngineProfile.EngineProfileSlug.String(), resolved.ResolvedEngineProfile.RegistrySlug.String(), nil
}
