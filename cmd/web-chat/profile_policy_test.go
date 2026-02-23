package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/stretchr/testify/require"
)

func TestWebChatProfileResolver_WS_DefaultProfile(t *testing.T) {
	profileRegistry, err := newInMemoryProfileRegistry(
		"default",
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("default"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"}},
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("agent"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are agent"}},
	)
	require.NoError(t, err)
	resolver := newWebChatProfileResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultWebChatRegistrySlug))

	req := httptest.NewRequest(http.MethodGet, "/ws?conv_id=conv-1", nil)
	plan, err := resolver.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "conv-1", plan.ConvID)
	require.Equal(t, "default", plan.RuntimeKey)
	require.Equal(t, "You are default", plan.Overrides["system_prompt"])
}

func TestWebChatProfileResolver_Chat_OverridePolicy(t *testing.T) {
	profileRegistry, err := newInMemoryProfileRegistry(
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
	resolver := newWebChatProfileResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultWebChatRegistrySlug))

	req := httptest.NewRequest(
		http.MethodPost,
		"/chat/default",
		bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-1","overrides":{"system_prompt":"override"}}`),
	)
	_, err = resolver.Resolve(req)
	require.Error(t, err)
	require.ErrorContains(t, err, "does not allow engine overrides")

	reqAllowed := httptest.NewRequest(
		http.MethodPost,
		"/chat/agent",
		bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-2","overrides":{"system_prompt":"override"}}`),
	)
	plan, err := resolver.Resolve(reqAllowed)
	require.NoError(t, err)
	require.Equal(t, "agent", plan.RuntimeKey)
	require.Equal(t, "override", plan.Overrides["system_prompt"])
}

func TestRegisterProfileHandlers_GetAndSetProfile(t *testing.T) {
	profileRegistry, err := newInMemoryProfileRegistry(
		"default",
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("default"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"}},
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("agent"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are agent"}},
	)
	require.NoError(t, err)
	resolver := newWebChatProfileResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultWebChatRegistrySlug))

	mux := http.NewServeMux()
	registerProfileHandlers(mux, resolver)

	reqList := httptest.NewRequest(http.MethodGet, "/api/chat/profiles", nil)
	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, reqList)
	require.Equal(t, http.StatusOK, recList.Code)

	var listed []map[string]any
	require.NoError(t, json.Unmarshal(recList.Body.Bytes(), &listed))
	require.Len(t, listed, 2)
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
