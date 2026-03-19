package helpers

import (
	"context"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

type ResolvedInferenceSettings struct {
	InferenceSettings     *aisettings.InferenceSettings
	ResolvedEngineProfile *gepprofiles.ResolvedEngineProfile
	ConfigFiles           []string
	Close                 func()
}

func ResolveFinalInferenceSettings(
	ctx context.Context,
	parsed *values.Values,
) (*ResolvedInferenceSettings, error) {
	resolved, err := ResolveCLIEngineSettings(ctx, parsed)
	if err != nil {
		return nil, err
	}
	return &ResolvedInferenceSettings{
		InferenceSettings:     resolved.FinalInferenceSettings,
		ResolvedEngineProfile: resolved.ResolvedEngineProfile,
		ConfigFiles:           append([]string(nil), resolved.ConfigFiles...),
		Close:                 resolved.Close,
	}, nil
}
