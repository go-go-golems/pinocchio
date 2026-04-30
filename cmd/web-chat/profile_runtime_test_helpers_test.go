package main

import (
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/stretchr/testify/require"
)

func testEngineProfileWithRuntime(t *testing.T, slug string, runtime *infruntime.ProfileRuntime) *gepprofiles.EngineProfile {
	t.Helper()

	profile := &gepprofiles.EngineProfile{
		Slug: gepprofiles.MustEngineProfileSlug(slug),
	}
	require.NoError(t, infruntime.SetProfileRuntime(profile, runtime))
	return profile
}
