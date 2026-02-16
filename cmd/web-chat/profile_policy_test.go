package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebChatProfileResolver_WS_DefaultProfile(t *testing.T) {
	profiles := newChatProfileRegistry(
		"default",
		&chatProfile{Slug: "default", DefaultPrompt: "You are default"},
		&chatProfile{Slug: "agent", DefaultPrompt: "You are agent"},
	)
	resolver := newWebChatProfileResolver(profiles)

	req := httptest.NewRequest(http.MethodGet, "/ws?conv_id=conv-1", nil)
	plan, err := resolver.Resolve(req)
	require.NoError(t, err)
	require.Equal(t, "conv-1", plan.ConvID)
	require.Equal(t, "default", plan.RuntimeKey)
	require.Equal(t, "You are default", plan.Overrides["system_prompt"])
}

func TestWebChatProfileResolver_Chat_OverridePolicy(t *testing.T) {
	profiles := newChatProfileRegistry(
		"default",
		&chatProfile{Slug: "default", DefaultPrompt: "You are default", AllowOverrides: false},
		&chatProfile{Slug: "agent", DefaultPrompt: "You are agent", AllowOverrides: true},
	)
	resolver := newWebChatProfileResolver(profiles)

	req := httptest.NewRequest(
		http.MethodPost,
		"/chat/default",
		bytes.NewBufferString(`{"prompt":"hi","conv_id":"conv-1","overrides":{"system_prompt":"override"}}`),
	)
	_, err := resolver.Resolve(req)
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
	profiles := newChatProfileRegistry(
		"default",
		&chatProfile{Slug: "default", DefaultPrompt: "You are default"},
		&chatProfile{Slug: "agent", DefaultPrompt: "You are agent"},
	)

	mux := http.NewServeMux()
	registerProfileHandlers(mux, profiles)

	reqList := httptest.NewRequest(http.MethodGet, "/api/chat/profiles", nil)
	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, reqList)
	require.Equal(t, http.StatusOK, recList.Code)

	var listed []map[string]any
	require.NoError(t, json.Unmarshal(recList.Body.Bytes(), &listed))
	require.Len(t, listed, 2)
	require.Equal(t, "default", listed[0]["slug"])
	require.Equal(t, "agent", listed[1]["slug"])

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
