package main

import (
	"fmt"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	aitypes "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
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

func testEngineProfileWithRuntimeAndInferenceSettings(t *testing.T, slug string, runtime *infruntime.ProfileRuntime, inferenceSettings *aisettings.InferenceSettings) *gepprofiles.EngineProfile {
	t.Helper()

	profile := testEngineProfileWithRuntime(t, slug, runtime)
	profile.InferenceSettings = inferenceSettings
	return profile
}

func testInferenceSettings(t *testing.T, apiType aitypes.ApiType, model string) *aisettings.InferenceSettings {
	t.Helper()

	ss, err := aisettings.NewInferenceSettings()
	require.NoError(t, err)
	ss.Chat.ApiType = &apiType
	ss.Chat.Engine = &model
	return ss
}

func testRegistryYAMLWithRuntime(registrySlug string, profileSlug string, systemPrompt string, version uint64) string {
	return fmt.Sprintf(`slug: %s
profiles:
  %s:
    slug: %s
    metadata:
      version: %d
    extensions:
      pinocchio.webchat_runtime@v1:
        system_prompt: %s
`, registrySlug, profileSlug, profileSlug, version, systemPrompt)
}
