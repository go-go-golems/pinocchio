package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/mockruntime"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/profiles"
	webchatruntime "github.com/go-go-golems/pinocchio/cmd/web-chat/internal/runtime"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/webapp"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/httpapi"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/plugins"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/stretchr/testify/require"
)

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

func newMigratedRuntimeTestServer(t *testing.T) (*httpapi.Server, *httptest.Server) {
	t.Helper()
	profileRegistry, err := profiles.NewInMemoryProfileService(
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}),
	)
	require.NoError(t, err)
	resolver := profiles.NewRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(profiles.DefaultRegistrySlug), nil)
	canonicalApp, err := httpapi.New(httpapi.WithRuntimeResolver(mockParityResolver()))
	require.NoError(t, err)
	appConfigJS, err := webapp.RuntimeConfigScript("")
	require.NoError(t, err)
	appFS := fstest.MapFS{
		"static/index.html":          {Data: []byte("<html><body>migrated ui</body></html>")},
		"static/dist/assets/app.js":  {Data: []byte("console.log('asset')")},
		"static/dist/index.html":     {Data: []byte("<html><body>built migrated ui</body></html>")},
		"static/dist/assets/app.css": {Data: []byte("body{}")},
	}
	mux := webapp.NewMux(webapp.MuxOptions{StaticFS: appFS, AppConfigJS: appConfigJS, RequestResolver: resolver, ChatServer: canonicalApp})
	httpSrv := httptest.NewServer(mux)
	t.Cleanup(func() {
		httpSrv.Close()
		_ = canonicalApp.Close()
	})
	return canonicalApp, httpSrv
}

func TestCanonicalRuntimeResolver_MockParityProfileShortCircuitsNormalComposer(t *testing.T) {
	profileRegistry, err := profiles.NewInMemoryProfileService(
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}),
	)
	require.NoError(t, err)
	requestResolver := profiles.NewRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(profiles.DefaultRegistrySlug), nil)
	composerCalled := false
	resolver := webchatruntime.NewCanonicalRuntimeResolver(requestResolver, infruntime.RuntimeBuilderFunc(func(ctx context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		composerCalled = true
		return infruntime.ComposedRuntime{}, nil
	}))

	composed, err := resolver.Resolve(context.Background(), httptest.NewRequest(http.MethodPost, "/api/chat/sessions/sess/messages", strings.NewReader(`{}`)), sessions.Info{ID: "sess"}, httpapi.MessageRequest{Profile: profiles.MockParityProfile})
	require.NoError(t, err)
	require.NotNil(t, composed)
	require.IsType(t, &mockruntime.Engine{}, composed.Engine)
	require.False(t, composerCalled)
}

func TestBuildAppMux_ServesCanonicalRoutesAndRemovesLegacyRoute(t *testing.T) {
	_, httpSrv := newMigratedRuntimeTestServer(t)

	indexResp, err := http.Get(httpSrv.URL + "/")
	require.NoError(t, err)
	defer func() { _ = indexResp.Body.Close() }()
	require.Equal(t, http.StatusOK, indexResp.StatusCode)
	indexBody := readBody(t, indexResp)
	require.Contains(t, indexBody, "built migrated ui")

	assetResp, err := http.Get(httpSrv.URL + "/assets/app.js")
	require.NoError(t, err)
	defer func() { _ = assetResp.Body.Close() }()
	require.Equal(t, http.StatusOK, assetResp.StatusCode)
	require.Contains(t, readBody(t, assetResp), "asset")

	configResp, err := http.Get(httpSrv.URL + "/app-config.js")
	require.NoError(t, err)
	defer func() { _ = configResp.Body.Close() }()
	require.Equal(t, http.StatusOK, configResp.StatusCode)
	require.Contains(t, readBody(t, configResp), `"basePrefix":""`)

	profilesResp, err := http.Get(httpSrv.URL + "/api/chat/profiles")
	require.NoError(t, err)
	defer func() { _ = profilesResp.Body.Close() }()
	require.Equal(t, http.StatusOK, profilesResp.StatusCode)
	profilesBody := readBody(t, profilesResp)
	require.Contains(t, profilesBody, "default")
	require.Contains(t, profilesBody, profiles.MockParityProfile)

	createResp, err := http.Post(httpSrv.URL+"/api/chat/sessions", "application/json", strings.NewReader(`{"profile":"default"}`))
	require.NoError(t, err)
	defer func() { _ = createResp.Body.Close() }()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	require.Contains(t, readBody(t, createResp), "sessionId")

	legacyResp, err := http.Post(httpSrv.URL+"/chat", "application/json", strings.NewReader(`{"prompt":"hello"}`))
	require.NoError(t, err)
	defer func() { _ = legacyResp.Body.Close() }()
	require.Equal(t, http.StatusNotFound, legacyResp.StatusCode)

	timelineResp, err := http.Get(httpSrv.URL + "/api/timeline?conv_id=conv-1")
	require.NoError(t, err)
	defer func() { _ = timelineResp.Body.Close() }()
	require.Equal(t, http.StatusNotFound, timelineResp.StatusCode)
}

func TestBuildRootHandler_MountsCanonicalAppUnderCustomRoot(t *testing.T) {
	profileRegistry, err := profiles.NewInMemoryProfileService(
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}),
	)
	require.NoError(t, err)
	resolver := profiles.NewRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(profiles.DefaultRegistrySlug), nil)
	canonicalApp, err := httpapi.New(httpapi.WithRuntimeResolver(mockParityResolver()))
	require.NoError(t, err)
	defer func() { _ = canonicalApp.Close() }()

	appConfigJS, err := webapp.RuntimeConfigScript("/chat")
	require.NoError(t, err)
	appFS := fstest.MapFS{
		"static/index.html": {Data: []byte("<html><body>rooted ui</body></html>")},
	}
	mux := webapp.NewMux(webapp.MuxOptions{StaticFS: appFS, AppConfigJS: appConfigJS, RequestResolver: resolver, ChatServer: canonicalApp})
	handler := webapp.MountRoot("/chat", mux, appConfigJS)
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	indexResp, err := http.Get(httpSrv.URL + "/chat/")
	require.NoError(t, err)
	defer func() { _ = indexResp.Body.Close() }()
	require.Equal(t, http.StatusOK, indexResp.StatusCode)
	require.Contains(t, readBody(t, indexResp), "rooted ui")

	configResp, err := http.Get(httpSrv.URL + "/app-config.js")
	require.NoError(t, err)
	defer func() { _ = configResp.Body.Close() }()
	require.Equal(t, http.StatusOK, configResp.StatusCode)
	require.Contains(t, readBody(t, configResp), `"basePrefix":"/chat"`)

	prefixedConfigResp, err := http.Get(httpSrv.URL + "/chat/app-config.js")
	require.NoError(t, err)
	defer func() { _ = prefixedConfigResp.Body.Close() }()
	require.Equal(t, http.StatusOK, prefixedConfigResp.StatusCode)
	require.Contains(t, readBody(t, prefixedConfigResp), `"basePrefix":"/chat"`)

	createResp, err := http.Post(httpSrv.URL+"/chat/api/chat/sessions", "application/json", strings.NewReader(`{"profile":"default"}`))
	require.NoError(t, err)
	defer func() { _ = createResp.Body.Close() }()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	unprefixedResp, err := http.Post(httpSrv.URL+"/api/chat/sessions", "application/json", strings.NewReader(`{"profile":"default"}`))
	require.NoError(t, err)
	defer func() { _ = unprefixedResp.Body.Close() }()
	require.Equal(t, http.StatusNotFound, unprefixedResp.StatusCode)
}

func mockParityResolver() httpapi.RuntimeResolver {
	return httpapi.RuntimeResolverFunc(func(context.Context, *http.Request, sessions.Info, httpapi.MessageRequest) (*infruntime.ComposedRuntime, error) {
		composed := mockruntime.NewComposedRuntime(mockruntime.Options{})
		return &composed, nil
	})
}

// The mock parity runtime streams reasoning, a backend tool call and text;
// through the HTTP API they project into chat-message and tool-call entities.
func TestMockParityRuntimeProjectsDeterministicEntitiesThroughHTTP(t *testing.T) {
	srv, err := httpapi.New(
		httpapi.WithAutoCreateSessions(true),
		httpapi.WithRuntimeResolver(mockParityResolver()),
		httpapi.WithChatPlugins(plugins.NewReasoningPlugin(), plugins.NewToolCallPlugin()),
	)
	require.NoError(t, err)
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		httpSrv.Close()
		_ = srv.Close()
	})

	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-mock/messages", "application/json", strings.NewReader(`{"prompt":"run deterministic parity","profile":"mock_parity"}`))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	deadline := time.Now().Add(3 * time.Second)
	for {
		snapResp, err := http.Get(httpSrv.URL + "/api/chat/sessions/sess-mock")
		require.NoError(t, err)
		var got httpapi.SessionResponse
		require.NoError(t, json.NewDecoder(snapResp.Body).Decode(&got))
		_ = snapResp.Body.Close()
		if !got.Running && got.Snapshot.Status == "finished" {
			kinds := map[string]bool{}
			texts := []string{}
			for _, entity := range got.Snapshot.Entities {
				kinds[entity.Kind] = true
				if payload, ok := entity.Payload.(map[string]any); ok {
					for _, key := range []string{"text", "content"} {
						if text, ok := payload[key].(string); ok {
							texts = append(texts, text)
						}
					}
				}
			}
			require.True(t, kinds[chatapp.TimelineEntityChatMessage])
			require.True(t, kinds[plugins.TimelineEntityToolCall])
			require.Contains(t, strings.Join(texts, "\n"), "Mock parity run complete")
			require.Contains(t, strings.Join(texts, "\n"), "Inspecting deterministic inputs")
			return
		}
		require.True(t, time.Now().Before(deadline), "timed out waiting for the mock parity run")
		time.Sleep(10 * time.Millisecond)
	}
}
