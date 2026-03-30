package runtime

import (
	"context"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	aitypes "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
	"github.com/stretchr/testify/require"
)

func TestMergeProfileRuntime_DefaultUnionMergesToolsAndReplacesMiddlewareByKey(t *testing.T) {
	base := &ProfileRuntime{
		SystemPrompt: "base",
		Middlewares: []MiddlewareUse{
			{Name: "agentmode", Config: map[string]any{"sanitize_yaml": true}},
			{Name: "audit", ID: "main", Config: map[string]any{"phase": "base"}},
		},
		Tools: []string{"search"},
	}
	overlay := &ProfileRuntime{
		SystemPrompt: "leaf",
		Middlewares: []MiddlewareUse{
			{Name: "audit", ID: "main", Config: map[string]any{"phase": "leaf"}},
		},
		Tools: []string{"summarize", "search"},
	}

	merged := MergeProfileRuntime(base, overlay, DefaultMergeProfileRuntimeOptions())
	require.NotNil(t, merged)
	require.Equal(t, "leaf", merged.SystemPrompt)
	require.Equal(t, []string{"search", "summarize"}, merged.Tools)
	require.Len(t, merged.Middlewares, 2)
	require.Equal(t, "leaf", merged.Middlewares[1].Config["phase"])
}

func TestMergeProfileRuntime_ReplaceToolsModeReplacesTools(t *testing.T) {
	merged := MergeProfileRuntime(
		&ProfileRuntime{Tools: []string{"search", "summarize"}},
		&ProfileRuntime{Tools: []string{"sql_query"}},
		MergeProfileRuntimeOptions{ToolMergeMode: ToolMergeModeReplace},
	)
	require.NotNil(t, merged)
	require.Equal(t, []string{"sql_query"}, merged.Tools)
}

func TestResolveRuntimePlan_MergesBaseRuntimeAndResolvedStackRuntimes(t *testing.T) {
	registry := newTestRuntimeRegistry(t,
		testRuntimeProfile(t, "base", &ProfileRuntime{
			SystemPrompt: "base prompt",
			Middlewares: []MiddlewareUse{
				{Name: "audit", ID: "main", Config: map[string]any{"phase": "base"}},
			},
			Tools: []string{"search"},
		}, nil),
		testRuntimeProfile(t, "leaf", &ProfileRuntime{
			SystemPrompt: "leaf prompt",
			Middlewares: []MiddlewareUse{
				{Name: "audit", ID: "main", Config: map[string]any{"phase": "leaf"}},
			},
			Tools: []string{"summarize"},
		}, testInferenceSettings(t, aitypes.ApiTypeOpenAIResponses, "gpt-5-mini")),
	)

	resolved := &gepprofiles.ResolvedEngineProfile{
		RegistrySlug:      gepprofiles.MustRegistrySlug("default"),
		EngineProfileSlug: gepprofiles.MustEngineProfileSlug("leaf"),
		InferenceSettings: testInferenceSettings(t, aitypes.ApiTypeOpenAIResponses, "gpt-5-mini"),
		StackLineage: []gepprofiles.ResolvedProfileStackEntry{
			{RegistrySlug: gepprofiles.MustRegistrySlug("default"), EngineProfileSlug: gepprofiles.MustEngineProfileSlug("base"), Version: 1},
			{RegistrySlug: gepprofiles.MustRegistrySlug("default"), EngineProfileSlug: gepprofiles.MustEngineProfileSlug("leaf"), Version: 2},
		},
		Metadata: map[string]any{
			"profile.version": uint64(2),
		},
	}

	plan, err := ResolveRuntimePlan(context.Background(), registry, resolved, ResolveRuntimePlanOptions{
		BaseRuntime: &ProfileRuntime{
			Middlewares: []MiddlewareUse{{Name: "agentmode"}},
		},
		BaseInferenceSettings: testInferenceSettings(t, aitypes.ApiTypeOpenAI, "base-model"),
	})
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, uint64(2), plan.ProfileVersion)
	require.NotNil(t, plan.Runtime)
	require.Equal(t, "leaf prompt", plan.Runtime.SystemPrompt)
	require.Equal(t, []string{"search", "summarize"}, plan.Runtime.Tools)
	require.Len(t, plan.Runtime.Middlewares, 2)
	require.Equal(t, "agentmode", plan.Runtime.Middlewares[0].Name)
	require.Equal(t, "leaf", plan.Runtime.Middlewares[1].Config["phase"])
	require.NotNil(t, plan.InferenceSettings)
	require.Equal(t, "gpt-5-mini", *plan.InferenceSettings.Chat.Engine)
}

func TestBuildRuntimeFingerprintFromSettings_UsesTypedPayload(t *testing.T) {
	fp := BuildRuntimeFingerprintFromSettings("default", 7, &ProfileRuntime{
		SystemPrompt: "hi",
		Middlewares:  []MiddlewareUse{{Name: "agentmode"}},
		Tools:        []string{"search"},
	}, testInferenceSettings(t, aitypes.ApiTypeOpenAI, "base-model"))

	require.Contains(t, fp, `"runtime_key":"default"`)
	require.Contains(t, fp, `"profile_version":7`)
	require.Contains(t, fp, `"tools":["search"]`)
}

func newTestRuntimeRegistry(t *testing.T, profiles ...*gepprofiles.EngineProfile) gepprofiles.Registry {
	t.Helper()
	store := gepprofiles.NewInMemoryEngineProfileStore()
	registry := &gepprofiles.EngineProfileRegistry{
		Slug:                     gepprofiles.MustRegistrySlug("default"),
		DefaultEngineProfileSlug: gepprofiles.MustEngineProfileSlug("leaf"),
		Profiles:                 map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{},
	}
	for _, profile := range profiles {
		registry.Profiles[profile.Slug] = profile
	}
	require.NoError(t, store.UpsertRegistry(context.Background(), registry, gepprofiles.SaveOptions{Actor: "test", Source: "test"}))
	svc, err := gepprofiles.NewStoreRegistry(store, gepprofiles.MustRegistrySlug("default"))
	require.NoError(t, err)
	return svc
}

func testRuntimeProfile(t *testing.T, slug string, runtime *ProfileRuntime, inference *aisettings.InferenceSettings) *gepprofiles.EngineProfile {
	t.Helper()
	profile := &gepprofiles.EngineProfile{
		Slug: gepprofiles.MustEngineProfileSlug(slug),
	}
	require.NoError(t, SetProfileRuntime(profile, runtime))
	profile.InferenceSettings = inference
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
