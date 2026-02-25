package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
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

	store := gepprofiles.NewInMemoryProfileStore()

	defaultRegistry := &gepprofiles.ProfileRegistry{
		Slug:               gepprofiles.MustRegistrySlug(defaultRegistrySlug),
		DefaultProfileSlug: gepprofiles.MustProfileSlug("default"),
		Profiles: map[gepprofiles.ProfileSlug]*gepprofiles.Profile{
			gepprofiles.MustProfileSlug("default"): {
				Slug:    gepprofiles.MustProfileSlug("default"),
				Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"},
				Metadata: gepprofiles.ProfileMetadata{
					Version: 1,
				},
			},
		},
	}
	require.NoError(t, gepprofiles.ValidateRegistry(defaultRegistry))
	require.NoError(t, store.UpsertRegistry(context.Background(), defaultRegistry, gepprofiles.SaveOptions{Actor: "tests", Source: "tests"}))

	teamRegistry := &gepprofiles.ProfileRegistry{
		Slug:               gepprofiles.MustRegistrySlug("team"),
		DefaultProfileSlug: gepprofiles.MustProfileSlug("analyst"),
		Profiles: map[gepprofiles.ProfileSlug]*gepprofiles.Profile{
			gepprofiles.MustProfileSlug("analyst"): {
				Slug:    gepprofiles.MustProfileSlug("analyst"),
				Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are analyst"},
				Metadata: gepprofiles.ProfileMetadata{
					Version: 7,
				},
			},
		},
	}
	require.NoError(t, gepprofiles.ValidateRegistry(teamRegistry))
	require.NoError(t, store.UpsertRegistry(context.Background(), teamRegistry, gepprofiles.SaveOptions{Actor: "tests", Source: "tests"}))

	profileRegistry, err := gepprofiles.NewStoreRegistry(store, gepprofiles.MustRegistrySlug(defaultRegistrySlug))
	require.NoError(t, err)
	return newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))
}

func TestWebChatProfileResolver_WS_DefaultProfile(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("default"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"}},
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("agent"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are agent"}},
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))

	req := httptest.NewRequest(http.MethodGet, "/ws?conv_id=conv-1", nil)
	plan, err := resolver.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "conv-1", plan.ConvID)
	require.Equal(t, "default", plan.RuntimeKey)
	require.Nil(t, plan.Overrides)
	require.NotNil(t, plan.ResolvedRuntime)
	require.Equal(t, "You are default", plan.ResolvedRuntime.SystemPrompt)
}

func TestWebChatProfileResolver_Chat_OverridePolicy(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("default"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"},
			Policy:  gepprofiles.PolicySpec{AllowOverrides: false},
		},
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("agent"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are agent"},
			Policy:  gepprofiles.PolicySpec{AllowOverrides: true},
		},
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))

	req := httptest.NewRequest(
		http.MethodPost,
		"/chat/default",
		bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-1","overrides":{"system_prompt":"override"}}`),
	)
	_, err = resolver.Resolve(req)
	require.Error(t, err)
	require.ErrorContains(t, err, "request overrides are disabled for this profile")

	reqAllowed := httptest.NewRequest(
		http.MethodPost,
		"/chat/agent",
		bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-2","overrides":{"system_prompt":"override"}}`),
	)
	plan, err := resolver.Resolve(reqAllowed)
	require.NoError(t, err)
	require.Equal(t, "agent", plan.RuntimeKey)
	require.Nil(t, plan.Overrides)
	require.NotNil(t, plan.ResolvedRuntime)
	require.Equal(t, "override", plan.ResolvedRuntime.SystemPrompt)
}

func TestRegisterProfileHandlers_GetAndSetProfile(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("default"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"}},
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("agent"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are agent"}},
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))

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

	reqSet := httptest.NewRequest(http.MethodPost, "/api/chat/profile", bytes.NewBufferString(`{"slug":"agent"}`))
	recSet := httptest.NewRecorder()
	mux.ServeHTTP(recSet, reqSet)
	require.Equal(t, http.StatusOK, recSet.Code)

	var setResp map[string]any
	require.NoError(t, json.Unmarshal(recSet.Body.Bytes(), &setResp))
	require.Equal(t, "agent", setResp["slug"])
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
	require.Equal(t, "agent", getResp["slug"])
}

func TestWebChatProfileResolver_Chat_BodyProfileAndRegistry(t *testing.T) {
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
	require.Equal(t, uint64(7), plan.ProfileVersion)
	require.Nil(t, plan.Overrides)
	require.NotNil(t, plan.ResolvedRuntime)
	require.Equal(t, "You are analyst", plan.ResolvedRuntime.SystemPrompt)
}

func TestWebChatProfileResolver_WS_QueryProfileAndRegistry(t *testing.T) {
	resolver := newTestResolverWithMultipleRegistries(t)

	req := httptest.NewRequest(http.MethodGet, "/ws?conv_id=conv-1&registry=team&profile=analyst", nil)
	plan, err := resolver.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "conv-1", plan.ConvID)
	require.Equal(t, "analyst", plan.RuntimeKey)
	require.Equal(t, uint64(7), plan.ProfileVersion)
	require.Nil(t, plan.Overrides)
}

func TestWebChatProfileResolver_Chat_InvalidRegistryInBody(t *testing.T) {
	resolver := newTestResolverWithMultipleRegistries(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/chat",
		bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-1","registry":"invalid registry!","profile":"default"}`),
	)
	_, err := resolver.Resolve(req)
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid registry")
}

func TestWebChatProfileResolver_Chat_UnknownRegistryReturnsNotFound(t *testing.T) {
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

func TestProfileAPI_CRUDLifecycle(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("default"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"},
		},
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))

	mux := http.NewServeMux()
	registerProfileAPIHandlers(mux, resolver)

	createReq := httptest.NewRequest(http.MethodPost, "/api/chat/profiles", bytes.NewBufferString(`{
		"slug":"analyst",
		"display_name":"Analyst",
		"description":"Team analyst profile",
		"runtime":{
			"system_prompt":"You are analyst",
			"middlewares":[{"name":"agentmode","id":"primary","config":{"default_mode":"chat"}}]
		},
		"policy":{"allow_overrides":true},
		"extensions":{"Vendor.Custom@V1":{"flags":[{"enabled":true}]}},
		"set_default":true
	}`))
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)
	created := decodeJSON[profileDocument](t, createRec)
	require.Equal(t, "default", created.Registry)
	require.Equal(t, "analyst", created.Slug)
	require.True(t, created.IsDefault)
	require.Equal(t, uint64(1), created.Metadata.Version)
	createdExt, ok := created.Extensions["vendor.custom@v1"].(map[string]any)
	require.True(t, ok)
	require.True(t, createdExt["flags"].([]any)[0].(map[string]any)["enabled"].(bool))
	middlewareExt, ok := created.Extensions["middleware.agentmode_config@v1"].(map[string]any)
	require.True(t, ok)
	instances, ok := middlewareExt["instances"].(map[string]any)
	require.True(t, ok)
	primaryCfg, ok := instances["id:primary"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "chat", primaryCfg["default_mode"])

	getReq := httptest.NewRequest(http.MethodGet, "/api/chat/profiles/analyst", nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)
	got := decodeJSON[profileDocument](t, getRec)
	require.Equal(t, "Analyst", got.DisplayName)
	require.Equal(t, "You are analyst", got.Runtime.SystemPrompt)
	_, ok = got.Extensions["vendor.custom@v1"]
	require.True(t, ok)

	listReq := httptest.NewRequest(http.MethodGet, "/api/chat/profiles", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)
	var listItems []map[string]any
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listItems))
	require.GreaterOrEqual(t, len(listItems), 2)
	require.Equal(t, "analyst", listItems[0]["slug"])
	_, hasListExtensions := listItems[0]["extensions"]
	require.True(t, hasListExtensions)

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/chat/profiles/analyst", bytes.NewBufferString(`{
		"display_name":"Analyst V2",
		"runtime":{"system_prompt":"You are analyst v2"},
		"extensions":{"webchat.starter_suggestions@v1":{"items":["hello","world"]}},
		"expected_version":1
	}`))
	patchRec := httptest.NewRecorder()
	mux.ServeHTTP(patchRec, patchReq)
	require.Equal(t, http.StatusOK, patchRec.Code)
	patched := decodeJSON[profileDocument](t, patchRec)
	require.Equal(t, "Analyst V2", patched.DisplayName)
	require.Equal(t, "You are analyst v2", patched.Runtime.SystemPrompt)
	require.Equal(t, uint64(2), patched.Metadata.Version)
	_, ok = patched.Extensions["webchat.starter_suggestions@v1"]
	require.True(t, ok)

	setDefaultReq := httptest.NewRequest(http.MethodPost, "/api/chat/profiles/default/default", nil)
	setDefaultRec := httptest.NewRecorder()
	mux.ServeHTTP(setDefaultRec, setDefaultReq)
	require.Equal(t, http.StatusOK, setDefaultRec.Code)
	setDefaultResp := decodeJSON[profileDocument](t, setDefaultRec)
	require.Equal(t, "default", setDefaultResp.Slug)
	require.True(t, setDefaultResp.IsDefault)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/chat/profiles/analyst?expected_version=2", nil)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	require.Equal(t, http.StatusNoContent, deleteRec.Code)

	getMissingReq := httptest.NewRequest(http.MethodGet, "/api/chat/profiles/analyst", nil)
	getMissingRec := httptest.NewRecorder()
	mux.ServeHTTP(getMissingRec, getMissingReq)
	require.Equal(t, http.StatusNotFound, getMissingRec.Code)
}

func TestProfileAPI_ErrorMappings(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("default"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"},
		},
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("readonly"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are readonly"},
			Policy:  gepprofiles.PolicySpec{ReadOnly: true},
		},
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))

	mux := http.NewServeMux()
	registerProfileAPIHandlers(mux, resolver)

	invalidSlugReq := httptest.NewRequest(http.MethodPost, "/api/chat/profiles", bytes.NewBufferString(`{"slug":"bad slug!"}`))
	invalidSlugRec := httptest.NewRecorder()
	mux.ServeHTTP(invalidSlugRec, invalidSlugReq)
	require.Equal(t, http.StatusBadRequest, invalidSlugRec.Code)

	readonlyPatchReq := httptest.NewRequest(http.MethodPatch, "/api/chat/profiles/readonly", bytes.NewBufferString(`{"display_name":"nope"}`))
	readonlyPatchRec := httptest.NewRecorder()
	mux.ServeHTTP(readonlyPatchRec, readonlyPatchReq)
	require.Equal(t, http.StatusForbidden, readonlyPatchRec.Code)

	conflictPatchReq := httptest.NewRequest(http.MethodPatch, "/api/chat/profiles/default", bytes.NewBufferString(`{
		"display_name":"Default v2",
		"expected_version":999
	}`))
	conflictPatchRec := httptest.NewRecorder()
	mux.ServeHTTP(conflictPatchRec, conflictPatchReq)
	require.Equal(t, http.StatusConflict, conflictPatchRec.Code)

	invalidExtensionReq := httptest.NewRequest(http.MethodPost, "/api/chat/profiles", bytes.NewBufferString(`{
		"slug":"badext",
		"extensions":{"bad key":{"value":true}}
	}`))
	invalidExtensionRec := httptest.NewRecorder()
	mux.ServeHTTP(invalidExtensionRec, invalidExtensionReq)
	require.Equal(t, http.StatusBadRequest, invalidExtensionRec.Code)

	unknownMiddlewareReq := httptest.NewRequest(http.MethodPost, "/api/chat/profiles", bytes.NewBufferString(`{
		"slug":"badmw",
		"runtime":{"middlewares":[{"name":"unknown_middleware"}]}
	}`))
	unknownMiddlewareRec := httptest.NewRecorder()
	mux.ServeHTTP(unknownMiddlewareRec, unknownMiddlewareReq)
	require.Equal(t, http.StatusBadRequest, unknownMiddlewareRec.Code)
	require.Contains(t, unknownMiddlewareRec.Body.String(), "unknown middleware")

	invalidMiddlewareConfigReq := httptest.NewRequest(http.MethodPost, "/api/chat/profiles", bytes.NewBufferString(`{
		"slug":"badmwconfig",
		"runtime":{"middlewares":[{"name":"agentmode","config":{"unknown":"x"}}]}
	}`))
	invalidMiddlewareConfigRec := httptest.NewRecorder()
	mux.ServeHTTP(invalidMiddlewareConfigRec, invalidMiddlewareConfigReq)
	require.Equal(t, http.StatusBadRequest, invalidMiddlewareConfigRec.Code)
	require.Contains(t, invalidMiddlewareConfigRec.Body.String(), "runtime.middlewares[0].config")

	missingRegistryReq := httptest.NewRequest(http.MethodGet, "/api/chat/profiles?registry=missing", nil)
	missingRegistryRec := httptest.NewRecorder()
	mux.ServeHTTP(missingRegistryRec, missingRegistryReq)
	require.Equal(t, http.StatusNotFound, missingRegistryRec.Code)

	invalidExpectedReq := httptest.NewRequest(http.MethodDelete, "/api/chat/profiles/default?expected_version=abc", nil)
	invalidExpectedRec := httptest.NewRecorder()
	mux.ServeHTTP(invalidExpectedRec, invalidExpectedReq)
	require.Equal(t, http.StatusBadRequest, invalidExpectedRec.Code)
}

func TestProfileAPI_SchemaEndpoints(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("default"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"},
		},
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))

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
	require.GreaterOrEqual(t, len(extensionSchemas), 3)
	keys := map[string]bool{}
	for _, item := range extensionSchemas {
		key, _ := item["key"].(string)
		keys[key] = true
		_, hasSchema := item["schema"]
		require.True(t, hasSchema)
	}
	require.True(t, keys["webchat.starter_suggestions@v1"])
	require.True(t, keys["middleware.agentmode_config@v1"])
	require.True(t, keys["middleware.sqlite_config@v1"])
}

func TestWebChatProfileResolver_ProfilePrecedence(t *testing.T) {
	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("default"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "default"}},
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("path"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "path"}},
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("body"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "body"}},
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("query"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "query"}},
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("runtime"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "runtime"}},
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("cookie"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "cookie"}},
	)
	require.NoError(t, err)
	resolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))

	tests := []struct {
		name   string
		path   string
		body   string
		cookie string
		want   string
	}{
		{
			name:   "path wins",
			path:   "/chat/path?profile=query&runtime=runtime",
			body:   `{"prompt":"hi","conv_id":"conv-path","profile":"body"}`,
			cookie: "cookie",
			want:   "path",
		},
		{
			name:   "body wins over query and cookie",
			path:   "/chat?profile=query&runtime=runtime",
			body:   `{"prompt":"hi","conv_id":"conv-body","profile":"body"}`,
			cookie: "cookie",
			want:   "body",
		},
		{
			name:   "profile query wins over runtime query and cookie",
			path:   "/chat?profile=query&runtime=runtime",
			body:   `{"prompt":"hi","conv_id":"conv-query"}`,
			cookie: "cookie",
			want:   "query",
		},
		{
			name:   "runtime query wins over cookie",
			path:   "/chat?runtime=runtime",
			body:   `{"prompt":"hi","conv_id":"conv-runtime"}`,
			cookie: "cookie",
			want:   "runtime",
		},
		{
			name:   "cookie wins over default",
			path:   "/chat",
			body:   `{"prompt":"hi","conv_id":"conv-cookie"}`,
			cookie: "cookie",
			want:   "cookie",
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

func TestWebChatProfileResolver_RegistryPrecedence_BodyOverQuery(t *testing.T) {
	resolver := newTestResolverWithMultipleRegistries(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/chat?registry=default",
		bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-1","registry":"team","profile":"analyst"}`),
	)
	plan, err := resolver.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "analyst", plan.RuntimeKey)
	require.Equal(t, uint64(7), plan.ProfileVersion)
}

func TestNewSQLiteProfileService_BootstrapAndReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "profiles.db")

	registry, cleanup, err := newSQLiteProfileService(
		"",
		dbPath,
		"default",
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("default"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"},
		},
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("agent"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are agent"},
		},
	)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	profiles_, err := registry.ListProfiles(context.Background(), gepprofiles.MustRegistrySlug(defaultRegistrySlug))
	require.NoError(t, err)
	require.Len(t, profiles_, 2)

	registryAgain, cleanupAgain, err := newSQLiteProfileService(
		"",
		dbPath,
		"default",
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("default"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"},
		},
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("agent"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are agent"},
		},
	)
	require.NoError(t, err)
	t.Cleanup(cleanupAgain)

	profilesAgain, err := registryAgain.ListProfiles(context.Background(), gepprofiles.MustRegistrySlug(defaultRegistrySlug))
	require.NoError(t, err)
	require.Len(t, profilesAgain, 2)
}
