package main

import (
	"fmt"
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
