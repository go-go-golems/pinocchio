package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	appserver "github.com/go-go-golems/pinocchio/cmd/web-chat/internal/appserver"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/mockruntime"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/profiles"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/stretchr/testify/require"
)

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

func newMigratedRuntimeTestServer(t *testing.T) (*appserver.Server, *httptest.Server) {
	t.Helper()
	profileRegistry, err := profiles.NewInMemoryProfileService(
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}),
	)
	require.NoError(t, err)
	resolver := profiles.NewRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(profiles.DefaultRegistrySlug), nil)
	canonicalApp, err := appserver.NewServer()
	require.NoError(t, err)
	appConfigJS, err := runtimeConfigScript("")
	require.NoError(t, err)
	appFS := fstest.MapFS{
		"static/index.html":          {Data: []byte("<html><body>migrated ui</body></html>")},
		"static/dist/assets/app.js":  {Data: []byte("console.log('asset')")},
		"static/dist/index.html":     {Data: []byte("<html><body>built migrated ui</body></html>")},
		"static/dist/assets/app.css": {Data: []byte("body{}")},
	}
	mux := buildAppMux(appFS, appConfigJS, resolver, canonicalApp)
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
	resolver := newCanonicalRuntimeResolver(requestResolver, infruntime.RuntimeBuilderFunc(func(ctx context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		composerCalled = true
		return infruntime.ComposedRuntime{}, nil
	}))

	composed, err := resolver.Resolve(context.Background(), httptest.NewRequest(http.MethodPost, "/api/chat/sessions/sess/messages", strings.NewReader(`{}`)), "sess", profiles.MockParityProfile, "")
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
	require.Equal(t, http.StatusOK, createResp.StatusCode)
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
	canonicalApp, err := appserver.NewServer()
	require.NoError(t, err)
	defer func() { _ = canonicalApp.Close() }()

	appConfigJS, err := runtimeConfigScript("/chat")
	require.NoError(t, err)
	appFS := fstest.MapFS{
		"static/index.html": {Data: []byte("<html><body>rooted ui</body></html>")},
	}
	mux := buildAppMux(appFS, appConfigJS, resolver, canonicalApp)
	handler := buildRootHandler("/chat", mux, appConfigJS)
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
	require.Equal(t, http.StatusOK, createResp.StatusCode)

	unprefixedResp, err := http.Post(httpSrv.URL+"/api/chat/sessions", "application/json", strings.NewReader(`{"profile":"default"}`))
	require.NoError(t, err)
	defer func() { _ = unprefixedResp.Body.Close() }()
	require.Equal(t, http.StatusNotFound, unprefixedResp.StatusCode)
}
