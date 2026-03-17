package tuidemo

import (
	"context"
	"fmt"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	pinhelpers "github.com/go-go-golems/pinocchio/pkg/cmds/helpers"
	"github.com/pkg/errors"
)

func ResolveStepSettings(ctx context.Context, profileSlug string, profileRegistries string) (*aisettings.StepSettings, func(), error) {
	base, _, err := pinhelpers.ResolveBaseStepSettings(nil)
	if err != nil {
		return nil, nil, err
	}

	profileSettings := pinhelpers.ResolveProfileSettings(nil)
	if v := strings.TrimSpace(profileSlug); v != "" {
		profileSettings.Profile = v
	}
	if v := strings.TrimSpace(profileRegistries); v != "" {
		profileSettings.ProfileRegistries = v
	}
	if profileSettings.ProfileRegistries == "" {
		if base.Chat == nil || base.Chat.Engine == nil || strings.TrimSpace(*base.Chat.Engine) == "" {
			return nil, nil, fmt.Errorf("no engine configured; set PINOCCHIO_* base settings or provide --profile-registries/--profile")
		}
		return base, nil, nil
	}

	specEntries, err := gepprofiles.ParseProfileRegistrySourceEntries(profileSettings.ProfileRegistries)
	if err != nil {
		return nil, nil, errors.Wrap(err, "parse profile registry sources")
	}
	specs, err := gepprofiles.ParseRegistrySourceSpecs(specEntries)
	if err != nil {
		return nil, nil, errors.Wrap(err, "parse profile registry specs")
	}
	chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(ctx, specs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "initialize profile registry")
	}

	input := gepprofiles.ResolveInput{BaseStepSettings: base}
	if profileSettings.Profile != "" {
		slug, err := gepprofiles.ParseProfileSlug(profileSettings.Profile)
		if err != nil {
			_ = chain.Close()
			return nil, nil, err
		}
		input.ProfileSlug = slug
	}

	resolved, err := chain.ResolveEffectiveProfile(ctx, input)
	if err != nil {
		_ = chain.Close()
		return nil, nil, err
	}
	return resolved.EffectiveStepSettings, func() {
		_ = chain.Close()
	}, nil
}
