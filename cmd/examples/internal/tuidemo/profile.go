package tuidemo

import (
	"context"

	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

func ResolveInferenceSettings(ctx context.Context, parsed *values.Values) (*aisettings.InferenceSettings, func(), error) {
	resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsed)
	if err != nil {
		return nil, nil, err
	}
	return resolved.FinalInferenceSettings, resolved.Close, nil
}
