package tuidemo

import (
	"context"
	"fmt"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	pinhelpers "github.com/go-go-golems/pinocchio/pkg/cmds/helpers"
	"github.com/pkg/errors"
)

func ResolveInferenceSettings(ctx context.Context, profileSlug string, profileRegistries []string) (*aisettings.InferenceSettings, func(), error) {
	base, _, err := pinhelpers.ResolveBaseInferenceSettings(nil)
	if err != nil {
		return nil, nil, err
	}

	profileSettings := pinhelpers.ResolveProfileSettings(nil)
	if v := strings.TrimSpace(profileSlug); v != "" {
		profileSettings.Profile = v
	}
	if len(profileRegistries) > 0 {
		profileSettings.ProfileRegistries = normalizeProfileRegistries(profileRegistries)
	}
	if len(profileSettings.ProfileRegistries) == 0 {
		if base.Chat == nil || base.Chat.Engine == nil || strings.TrimSpace(*base.Chat.Engine) == "" {
			return nil, nil, fmt.Errorf("no engine configured; set PINOCCHIO_* base settings or provide --profile-registries/--profile")
		}
		return base, nil, nil
	}

	specs, err := gepprofiles.ParseRegistrySourceSpecs(profileSettings.ProfileRegistries)
	if err != nil {
		return nil, nil, errors.Wrap(err, "parse profile registry specs")
	}
	chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(ctx, specs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "initialize profile registry")
	}

	input := gepprofiles.ResolveInput{}
	if profileSettings.Profile != "" {
		slug, err := gepprofiles.ParseEngineProfileSlug(profileSettings.Profile)
		if err != nil {
			_ = chain.Close()
			return nil, nil, err
		}
		input.EngineProfileSlug = slug
	}

	resolved, err := chain.ResolveEngineProfile(ctx, input)
	if err != nil {
		_ = chain.Close()
		return nil, nil, err
	}
	finalSettings, err := gepprofiles.MergeInferenceSettings(base, resolved.InferenceSettings)
	if err != nil {
		_ = chain.Close()
		return nil, nil, errors.Wrap(err, "merge base inference settings with engine profile")
	}
	return finalSettings, func() {
		_ = chain.Close()
	}, nil
}

func normalizeProfileRegistries(entries []string) []string {
	ret := make([]string, 0, len(entries))
	for _, entry := range entries {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			ret = append(ret, trimmed)
		}
	}
	return ret
}
