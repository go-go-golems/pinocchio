package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/stretchr/testify/require"
)

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

func newTestResolverWithMultipleRegistries(t *testing.T) *ProfileRequestResolver {
	t.Helper()

	tmpDir := t.TempDir()
	defaultPath := filepath.Join(tmpDir, "default.yaml")
	teamPath := filepath.Join(tmpDir, "team.yaml")

	require.NoError(t, os.WriteFile(defaultPath, []byte(testRegistryYAMLWithRuntime("default", "default", "You are default", 1)), 0o644))
	require.NoError(t, os.WriteFile(teamPath, []byte(testRegistryYAMLWithRuntime("team", "analyst", "You are analyst", 7)), 0o644))

	specs, err := gepprofiles.ParseRegistrySourceSpecs([]string{defaultPath, teamPath})
	require.NoError(t, err)
	chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(context.Background(), specs)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = chain.Close()
	})
	return newProfileRequestResolver(chain, gepprofiles.MustRegistrySlug(defaultRegistrySlug), nil)
}

func newTestResolverWithDuplicateSlugAcrossRegistries(t *testing.T) *ProfileRequestResolver {
	t.Helper()

	tmpDir := t.TempDir()
	defaultPath := filepath.Join(tmpDir, "default.yaml")
	teamPath := filepath.Join(tmpDir, "team.yaml")

	require.NoError(t, os.WriteFile(defaultPath, []byte(testRegistryYAMLWithRuntime("default", "analyst", "You are default analyst", 1)), 0o644))
	require.NoError(t, os.WriteFile(teamPath, []byte(testRegistryYAMLWithRuntime("team", "analyst", "You are team analyst", 7)), 0o644))

	specs, err := gepprofiles.ParseRegistrySourceSpecs([]string{defaultPath, teamPath})
	require.NoError(t, err)
	chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(context.Background(), specs)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = chain.Close()
	})
	return newProfileRequestResolver(chain, gepprofiles.MustRegistrySlug(defaultRegistrySlug), nil)
}

func TestWebChatProfileResolver_WS_DefaultProfile(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}),
		testEngineProfileWithRuntime(t, "agent", &infruntime.ProfileRuntime{SystemPrompt: "You are agent"}),
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug), nil)

	req := httptest.NewRequest(http.MethodGet, "/ws?conv_id=conv-1", nil)
	plan, err := resolver.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "conv-1", plan.ConvID)
	require.Equal(t, "default", plan.RuntimeKey)
	require.NotEmpty(t, strings.TrimSpace(plan.RuntimeFingerprint))
	require.NotNil(t, plan.ResolvedRuntime)
	require.Equal(t, "You are default", plan.ResolvedRuntime.SystemPrompt)
	require.NotNil(t, plan.ProfileMetadata)
	_, hasLineage := plan.ProfileMetadata["profile.stack.lineage"]
	require.True(t, hasLineage)
}

func TestRegisterProfileHandlers_GetAndSetProfile(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}),
		testEngineProfileWithRuntime(t, "agent", &infruntime.ProfileRuntime{SystemPrompt: "You are agent"}),
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug), nil)

	mux := http.NewServeMux()
	registerProfileAPIHandlers(mux, resolver)

	reqList := httptest.NewRequest(http.MethodGet, "/api/chat/profiles", nil)
	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, reqList)
	require.Equal(t, http.StatusOK, recList.Code)

	var listed []map[string]any
	require.NoError(t, json.Unmarshal(recList.Body.Bytes(), &listed))
	require.Len(t, listed, 2)
	require.Equal(t, "agent", listed[0]["slug"])
	require.Equal(t, "default", listed[1]["slug"])
	slugs := map[string]bool{}
	for _, item := range listed {
		slug, _ := item["slug"].(string)
		slugs[slug] = true
	}
	require.True(t, slugs["default"])
	require.True(t, slugs["agent"])

	reqSet := httptest.NewRequest(http.MethodPost, "/api/chat/profile", bytes.NewBufferString(`{"profile":"agent","registry":"default"}`))
	recSet := httptest.NewRecorder()
	mux.ServeHTTP(recSet, reqSet)
	require.Equal(t, http.StatusOK, recSet.Code)

	var setResp map[string]any
	require.NoError(t, json.Unmarshal(recSet.Body.Bytes(), &setResp))
	require.Equal(t, "agent", setResp["profile"])
	require.Equal(t, "default", setResp["registry"])
	cookies := recSet.Result().Cookies()
	require.NotEmpty(t, cookies)
	require.True(t, cookies[0].Secure)
	require.True(t, cookies[0].HttpOnly)

	reqGet := httptest.NewRequest(http.MethodGet, "/api/chat/profile", nil)
	reqGet.AddCookie(cookies[0])
	recGet := httptest.NewRecorder()
	mux.ServeHTTP(recGet, reqGet)
	require.Equal(t, http.StatusOK, recGet.Code)

	var getResp map[string]any
	require.NoError(t, json.Unmarshal(recGet.Body.Bytes(), &getResp))
	require.Equal(t, "agent", getResp["profile"])
	require.Equal(t, "default", getResp["registry"])
}

func TestProfileAPI_ListAndGetAcrossLoadedRegistriesWhenRegistryUnset(t *testing.T) {
	resolver := newTestResolverWithMultipleRegistries(t)

	mux := http.NewServeMux()
	registerProfileAPIHandlers(mux, resolver)

	reqList := httptest.NewRequest(http.MethodGet, "/api/chat/profiles", nil)
	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, reqList)
	require.Equal(t, http.StatusOK, recList.Code)

	var listed []map[string]any
	require.NoError(t, json.Unmarshal(recList.Body.Bytes(), &listed))
	require.Len(t, listed, 2)

	seen := map[string]bool{}
	seenRegistries := map[string]bool{}
	for _, item := range listed {
		slug, _ := item["slug"].(string)
		registry, _ := item["registry"].(string)
		seen[slug] = true
		seenRegistries[registry] = true
	}
	require.True(t, seen["default"])
	require.True(t, seen["analyst"])
	require.True(t, seenRegistries["default"])
	require.True(t, seenRegistries["team"])

	reqGet := httptest.NewRequest(http.MethodGet, "/api/chat/profiles/analyst", nil)
	recGet := httptest.NewRecorder()
	mux.ServeHTTP(recGet, reqGet)
	require.Equal(t, http.StatusOK, recGet.Code)
	doc := decodeJSON[profileDocument](t, recGet)
	require.Equal(t, "team", doc.Registry)
	require.Equal(t, "analyst", doc.Slug)
}

func TestProfileAPI_ListAcrossRegistries_PreservesDuplicateSlugsWithRegistryIdentifier(t *testing.T) {
	resolver := newTestResolverWithDuplicateSlugAcrossRegistries(t)

	mux := http.NewServeMux()
	registerProfileAPIHandlers(mux, resolver)

	reqList := httptest.NewRequest(http.MethodGet, "/api/chat/profiles", nil)
	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, reqList)
	require.Equal(t, http.StatusOK, recList.Code)

	var listed []map[string]any
	require.NoError(t, json.Unmarshal(recList.Body.Bytes(), &listed))
	require.Len(t, listed, 2)

	analystRegistries := map[string]bool{}
	for _, item := range listed {
		slug, _ := item["slug"].(string)
		registry, _ := item["registry"].(string)
		require.Equal(t, "analyst", slug)
		analystRegistries[registry] = true
	}
	require.True(t, analystRegistries["default"])
	require.True(t, analystRegistries["team"])
}

func TestWebChatProfileResolver_Chat_BodyProfileAcrossStack(t *testing.T) {
	resolver := newTestResolverWithMultipleRegistries(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/chat",
		bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-1","registry":"team","profile":"analyst"}`),
	)
	plan, err := resolver.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "conv-1", plan.ConvID)
	require.Equal(t, "analyst", plan.RuntimeKey)
	require.NotEmpty(t, strings.TrimSpace(plan.RuntimeFingerprint))
	require.Equal(t, uint64(7), plan.ProfileVersion)
	require.NotNil(t, plan.ResolvedRuntime)
	require.Equal(t, "You are analyst", plan.ResolvedRuntime.SystemPrompt)
	require.NotNil(t, plan.ProfileMetadata)
	_, hasLineage := plan.ProfileMetadata["profile.stack.lineage"]
	require.True(t, hasLineage)
}

func TestBuildConversationPlan_BuildsLocalRuntimeBeforeTransportConversion(t *testing.T) {
	resolver := newTestResolverWithMultipleRegistries(t)
	resolvedProfile, err := resolver.resolveEffectiveProfile(context.Background(), gepprofiles.MustRegistrySlug("team"), gepprofiles.MustEngineProfileSlug("analyst"))
	require.NoError(t, err)

	plan, err := resolver.buildConversationPlan(context.Background(), "conv-1", "hi", "idem-1", resolvedProfile)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, "conv-1", plan.ConvID)
	require.NotNil(t, plan.Runtime)
	require.Equal(t, "analyst", plan.Runtime.RuntimeKey)
	require.NotEmpty(t, strings.TrimSpace(plan.Runtime.RuntimeFingerprint))
	require.Equal(t, uint64(7), plan.Runtime.ProfileVersion)
	require.Equal(t, "You are analyst", plan.Runtime.SystemPrompt)
	require.NotNil(t, plan.Runtime.ProfileMetadata)
	_, hasLineage := plan.Runtime.ProfileMetadata["profile.stack.lineage"]
	require.True(t, hasLineage)

	transport := toResolvedConversationRequest(plan)
	require.Equal(t, "analyst", transport.RuntimeKey)
	require.NotNil(t, transport.ResolvedRuntime)
	require.Equal(t, "You are analyst", transport.ResolvedRuntime.SystemPrompt)
	require.Equal(t, plan.Runtime.InferenceSettings == nil, transport.ResolvedInferenceSettings == nil)
}

func TestWebChatProfileResolver_WS_QueryProfileAcrossStack(t *testing.T) {
	resolver := newTestResolverWithMultipleRegistries(t)

	req := httptest.NewRequest(http.MethodGet, "/ws?conv_id=conv-1&registry=team&profile=analyst", nil)
	plan, err := resolver.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "conv-1", plan.ConvID)
	require.Equal(t, "analyst", plan.RuntimeKey)
	require.NotEmpty(t, strings.TrimSpace(plan.RuntimeFingerprint))
	require.Equal(t, uint64(7), plan.ProfileVersion)
}

func TestWebChatProfileResolver_Chat_InvalidRegistryInBodyReturnsBadRequest(t *testing.T) {
	resolver := newTestResolverWithMultipleRegistries(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/chat",
		bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-1","registry":"invalid registry!","profile":"default"}`),
	)
	_, err := resolver.Resolve(req)
	require.Error(t, err)
	var re *webhttp.RequestResolutionError
	require.ErrorAs(t, err, &re)
	require.Equal(t, http.StatusBadRequest, re.Status)
}

func TestWebChatProfileResolver_Chat_UnknownRegistryQueryReturnsNotFound(t *testing.T) {
	resolver := newTestResolverWithMultipleRegistries(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/chat?registry=missing",
		bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-1"}`),
	)
	_, err := resolver.Resolve(req)
	require.Error(t, err)
	var re *webhttp.RequestResolutionError
	require.ErrorAs(t, err, &re)
	require.Equal(t, http.StatusNotFound, re.Status)
}

func TestProfileAPI_SchemaEndpoints(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}),
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug), nil)

	mux := http.NewServeMux()
	registerProfileAPIHandlers(mux, resolver)

	middlewareReq := httptest.NewRequest(http.MethodGet, "/api/chat/schemas/middlewares", nil)
	middlewareRec := httptest.NewRecorder()
	mux.ServeHTTP(middlewareRec, middlewareReq)
	require.Equal(t, http.StatusOK, middlewareRec.Code)
	var middlewareSchemas []map[string]any
	require.NoError(t, json.Unmarshal(middlewareRec.Body.Bytes(), &middlewareSchemas))
	require.GreaterOrEqual(t, len(middlewareSchemas), 2)
	names := map[string]bool{}
	for _, item := range middlewareSchemas {
		if name, ok := item["name"].(string); ok {
			names[name] = true
		}
		version, hasVersion := item["version"].(float64)
		require.True(t, hasVersion)
		require.Equal(t, float64(1), version)
		displayName, hasDisplayName := item["display_name"].(string)
		require.True(t, hasDisplayName)
		require.NotEmpty(t, strings.TrimSpace(displayName))
		_, hasSchema := item["schema"]
		require.True(t, hasSchema)
	}
	require.True(t, names["agentmode"])
	require.True(t, names["sqlite"])

	extensionReq := httptest.NewRequest(http.MethodGet, "/api/chat/schemas/extensions", nil)
	extensionRec := httptest.NewRecorder()
	mux.ServeHTTP(extensionRec, extensionReq)
	require.Equal(t, http.StatusOK, extensionRec.Code)
	var extensionSchemas []map[string]any
	require.NoError(t, json.Unmarshal(extensionRec.Body.Bytes(), &extensionSchemas))
	require.GreaterOrEqual(t, len(extensionSchemas), 1)
	keys := map[string]bool{}
	for _, item := range extensionSchemas {
		key, _ := item["key"].(string)
		keys[key] = true
		_, hasSchema := item["schema"]
		require.True(t, hasSchema)
	}
	require.True(t, keys["webchat.starter_suggestions@v1"])
}

func TestWebChatProfileResolver_ProfilePrecedence(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "default"}),
		testEngineProfileWithRuntime(t, "path", &infruntime.ProfileRuntime{SystemPrompt: "path"}),
		testEngineProfileWithRuntime(t, "body", &infruntime.ProfileRuntime{SystemPrompt: "body"}),
		testEngineProfileWithRuntime(t, "query", &infruntime.ProfileRuntime{SystemPrompt: "query"}),
		testEngineProfileWithRuntime(t, "runtime", &infruntime.ProfileRuntime{SystemPrompt: "runtime"}),
		testEngineProfileWithRuntime(t, "cookie", &infruntime.ProfileRuntime{SystemPrompt: "cookie"}),
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug), nil)

	tests := []struct {
		name   string
		path   string
		body   string
		cookie string
		want   string
	}{
		{
			name:   "path wins",
			path:   "/chat/path?profile=query",
			body:   `{"prompt":"hi","conv_id":"conv-path","profile":"body"}`,
			cookie: "default/cookie",
			want:   "path",
		},
		{
			name:   "body profile wins over query and cookie",
			path:   "/chat?profile=query",
			body:   `{"prompt":"hi","conv_id":"conv-body","profile":"body"}`,
			cookie: "default/cookie",
			want:   "body",
		},
		{
			name:   "profile query wins over cookie",
			path:   "/chat?profile=query",
			body:   `{"prompt":"hi","conv_id":"conv-query"}`,
			cookie: "default/cookie",
			want:   "query",
		},
		{
			name:   "profile query wins over cookie with runtime-shaped value",
			path:   "/chat?profile=runtime",
			body:   `{"prompt":"hi","conv_id":"conv-runtime"}`,
			cookie: "default/cookie",
			want:   "runtime",
		},
		{
			name:   "cookie wins over default",
			path:   "/chat",
			body:   `{"prompt":"hi","conv_id":"conv-cookie"}`,
			cookie: "default/cookie",
			want:   "cookie",
		},
		{
			name:   "legacy cookie wins over default",
			path:   "/chat",
			body:   `{"prompt":"hi","conv_id":"conv-legacy-cookie"}`,
			cookie: "cookie",
			want:   "cookie",
		},
		{
			name:   "missing legacy cookie falls back to default",
			path:   "/chat",
			body:   `{"prompt":"hi","conv_id":"conv-missing-legacy-cookie"}`,
			cookie: "missing",
			want:   "default",
		},
		{
			name: "default fallback",
			path: "/chat",
			body: `{"prompt":"hi","conv_id":"conv-default"}`,
			want: "default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: currentProfileCookieName, Value: tc.cookie})
			}
			plan, err := resolver.Resolve(req)
			require.NoError(t, err)
			require.Equal(t, tc.want, plan.RuntimeKey)
		})
	}
}

func TestWebChatProfileResolver_LegacySelectorInputsReturnBadRequest(t *testing.T) {
	resolver := newTestResolverWithMultipleRegistries(t)

	req := httptest.NewRequest(http.MethodPost, "/chat?registry_slug=default", bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-1"}`))
	_, err := resolver.Resolve(req)
	require.Error(t, err)
	var re *webhttp.RequestResolutionError
	require.ErrorAs(t, err, &re)
	require.Equal(t, http.StatusBadRequest, re.Status)

	req = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-1","runtime_key":"analyst"}`))
	_, err = resolver.Resolve(req)
	require.Error(t, err)
	require.ErrorAs(t, err, &re)
	require.Equal(t, http.StatusBadRequest, re.Status)
}

func TestNewSQLiteProfileService_BootstrapAndReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "profiles.db")

	registry, cleanup, err := newSQLiteProfileService(
		"",
		dbPath,
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}),
		testEngineProfileWithRuntime(t, "agent", &infruntime.ProfileRuntime{SystemPrompt: "You are agent"}),
	)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	profiles_, err := registry.ListEngineProfiles(context.Background(), gepprofiles.MustRegistrySlug(defaultRegistrySlug))
	require.NoError(t, err)
	require.Len(t, profiles_, 2)

	registryAgain, cleanupAgain, err := newSQLiteProfileService(
		"",
		dbPath,
		"default",
		testEngineProfileWithRuntime(t, "default", &infruntime.ProfileRuntime{SystemPrompt: "You are default"}),
		testEngineProfileWithRuntime(t, "agent", &infruntime.ProfileRuntime{SystemPrompt: "You are agent"}),
	)
	require.NoError(t, err)
	t.Cleanup(cleanupAgain)

	profilesAgain, err := registryAgain.ListEngineProfiles(context.Background(), gepprofiles.MustRegistrySlug(defaultRegistrySlug))
	require.NoError(t, err)
	require.Len(t, profilesAgain, 2)
}
