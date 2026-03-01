package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/stretchr/testify/require"
)

func resetTimelineRuntimeLoaderState(t *testing.T) {
	t.Helper()
	webchat.ClearTimelineHandlers()
	webchat.RegisterDefaultTimelineHandlers()
	webchat.ClearTimelineRuntime()
	t.Cleanup(func() {
		webchat.ClearTimelineHandlers()
		webchat.RegisterDefaultTimelineHandlers()
		webchat.ClearTimelineRuntime()
	})
}

func semFrameForLoaderTest(t *testing.T, eventType, id string, seq uint64, data map[string]any) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"sem": true,
		"event": map[string]any{
			"type": eventType,
			"id":   id,
			"seq":  seq,
			"data": data,
		},
	})
	require.NoError(t, err)
	return payload
}

func TestNormalizeTimelineJSScriptPaths(t *testing.T) {
	paths := normalizeTimelineJSScriptPaths([]string{" ./a.js, ./b.js ", "", " ./c.js "})
	require.Equal(t, []string{"./a.js", "./b.js", "./c.js"}, paths)
}

func TestConfigureTimelineJSScripts_LoadsRuntimeAndProjectsEvents(t *testing.T) {
	resetTimelineRuntimeLoaderState(t)

	script := `
const p = require("pinocchio");
p.timeline.registerSemReducer("llm.delta", function(ev) {
  return {
    consume: true,
    upserts: [{
      id: ev.id + "-loader",
      kind: "js.loader",
      props: { cumulative: ev.data && ev.data.cumulative }
    }]
  };
});
`
	scriptPath := filepath.Join(t.TempDir(), "loader.js")
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o600))

	require.NoError(t, configureTimelineJSScripts([]string{scriptPath}))

	store := chatstore.NewInMemoryTimelineStore(100)
	projector := webchat.NewTimelineProjector("conv-loader-test", store, nil)
	require.NoError(t, projector.ApplySemFrame(context.Background(), semFrameForLoaderTest(t, "llm.delta", "evt-1", 1, map[string]any{
		"delta":      "h",
		"cumulative": "hello",
	})))

	snap, err := store.GetSnapshot(context.Background(), "conv-loader-test", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)
	require.Equal(t, "evt-1-loader", snap.Entities[0].Id)
	require.Equal(t, "js.loader", snap.Entities[0].Kind)
	props := snap.Entities[0].GetProps().AsMap()
	require.Equal(t, "hello", props["cumulative"])
}

func TestConfigureTimelineJSScripts_ReturnsHelpfulErrorForMissingScript(t *testing.T) {
	resetTimelineRuntimeLoaderState(t)

	err := configureTimelineJSScripts([]string{"/tmp/does-not-exist.js"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "load timeline JS script")
}

func TestProfileResolver_GPT5NanoProfileIsResolvedForChatRequest(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{
			Slug: gepprofiles.MustProfileSlug("gpt-5-nano"),
			Runtime: gepprofiles.RuntimeSpec{
				StepSettingsPatch: map[string]any{
					"ai-chat": map[string]any{
						"ai-api-type": "openai-responses",
						"ai-engine":   "gpt-5-nano",
					},
				},
			},
		},
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))

	req := httptest.NewRequest("POST", "/chat/gpt-5-nano", strings.NewReader(`{"prompt":"hello","conv_id":"conv-gpt5nano"}`))
	resolved, err := resolver.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "gpt-5-nano", resolved.RuntimeKey)
	require.NotNil(t, resolved.ResolvedRuntime)
	aiChat, ok := resolved.ResolvedRuntime.StepSettingsPatch["ai-chat"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "gpt-5-nano", aiChat["ai-engine"])
}
