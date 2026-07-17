//go:build !windows

package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	"github.com/go-go-golems/pinocchio/pkg/oauthprofiles"
	"github.com/stretchr/testify/require"
)

func TestRunLoginExchangesPKCECodeAndSavesCredential(t *testing.T) {
	var tokenCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalls++
			require.NoError(t, r.ParseForm())
			require.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			require.Equal(t, "browser-code", r.Form.Get("code"))
			require.NotEmpty(t, r.Form.Get("code_verifier"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	profile, store, request := loginProfileFixture(t, server.URL)
	deps := defaultLoginDependencies()
	deps.openBrowser = func(rawURL string) error {
		parsed, err := url.Parse(rawURL)
		require.NoError(t, err)
		query := parsed.Query()
		require.Equal(t, "S256", query.Get("code_challenge_method"))
		require.NotEmpty(t, query.Get("code_challenge"))
		require.NotEmpty(t, query.Get("state"))
		callback, err := url.Parse(query.Get("redirect_uri"))
		require.NoError(t, err)
		callbackQuery := callback.Query()
		callbackQuery.Set("state", query.Get("state"))
		callbackQuery.Set("code", "browser-code")
		callback.RawQuery = callbackQuery.Encode()
		response, err := http.Get(callback.String())
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		return nil
	}

	require.NoError(t, runLogin(context.Background(), profile, time.Second, deps))
	require.Equal(t, 1, tokenCalls)
	credential, err := store.Load(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "new-access", credential.AccessToken)
	require.Equal(t, "new-refresh", credential.RefreshToken)
	require.False(t, credential.ExpiresAt.IsZero())
}

func TestRunLoginRejectsStateMismatchWithoutTokenLeak(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	profile, _, _ := loginProfileFixture(t, server.URL)
	deps := defaultLoginDependencies()
	deps.openBrowser = func(rawURL string) error {
		parsed, err := url.Parse(rawURL)
		require.NoError(t, err)
		callback, err := url.Parse(parsed.Query().Get("redirect_uri"))
		require.NoError(t, err)
		query := callback.Query()
		query.Set("state", "wrong-state")
		query.Set("code", "must-not-leak")
		callback.RawQuery = query.Encode()
		response, err := http.Get(callback.String())
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
		return nil
	}

	err := runLogin(context.Background(), profile, time.Second, deps)
	require.EqualError(t, err, "OAuth callback state did not match")
	require.NotContains(t, err.Error(), "must-not-leak")
}

func TestRunLoginRejectsProviderErrorAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	profile, _, _ := loginProfileFixture(t, server.URL)

	providerErrorDeps := defaultLoginDependencies()
	providerErrorDeps.openBrowser = func(rawURL string) error {
		parsed, err := url.Parse(rawURL)
		require.NoError(t, err)
		callback, err := url.Parse(parsed.Query().Get("redirect_uri"))
		require.NoError(t, err)
		query := callback.Query()
		query.Set("state", parsed.Query().Get("state"))
		query.Set("error", "access_denied")
		callback.RawQuery = query.Encode()
		response, err := http.Get(callback.String())
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
		return nil
	}
	require.EqualError(t, runLogin(context.Background(), profile, time.Second, providerErrorDeps), "OAuth provider returned an authorization error")

	timeoutDeps := defaultLoginDependencies()
	timeoutDeps.openBrowser = func(string) error { return nil }
	require.EqualError(t, runLogin(context.Background(), profile, 10*time.Millisecond, timeoutDeps), "OAuth browser callback timed out or was cancelled")
}

func TestCallbackHandlerRejectsWrongPathAndReplay(t *testing.T) {
	result := make(chan callbackResult, 1)
	handler := callbackHandler("expected-state", result)

	wrongPath := httptest.NewRecorder()
	handler.ServeHTTP(wrongPath, httptest.NewRequest(http.MethodGet, "/other", nil))
	require.Equal(t, http.StatusNotFound, wrongPath.Code)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, callbackPath+"?state=expected-state&code=code", nil))
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, "code", (<-result).code)

	replay := httptest.NewRecorder()
	handler.ServeHTTP(replay, httptest.NewRequest(http.MethodGet, callbackPath+"?state=expected-state&code=second", nil))
	require.Equal(t, http.StatusConflict, replay.Code)
}

func loginProfileFixture(t *testing.T, issuerURL string) (*profilebootstrap.ResolvedOAuthProfile, *oauthprofiles.YAMLStore, credentials.Request) {
	t.Helper()
	registrySlug := gepprofiles.MustRegistrySlug("workspace")
	profileSlug := gepprofiles.MustEngineProfileSlug("assistant")
	request := credentials.Request{Provider: "openai", BaseURL: "https://provider.example.test/v1"}
	extensions := map[string]any{
		oauthprofiles.ExtensionKey: map[string]any{
			"kind":                 oauthprofiles.OAuthBearerKind,
			"authorization_url":    issuerURL + "/authorize",
			"token_url":            issuerURL + "/token",
			"client_id":            "public-client",
			"refresh_token_policy": "preserve_previous",
		},
	}
	registry := &gepprofiles.EngineProfileRegistry{
		Slug:                     registrySlug,
		DefaultEngineProfileSlug: profileSlug,
		Profiles: map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{
			profileSlug: {Slug: profileSlug, Extensions: extensions},
		},
	}
	data, err := gepprofiles.EncodeEngineProfileYAMLSingleRegistry(registry)
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	path := filepath.Join(dir, "profiles.yaml")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	store, err := oauthprofiles.NewYAMLStore(path, registrySlug, profileSlug, request)
	require.NoError(t, err)
	oauthProfile, err := oauthprofiles.Parse(extensions)
	require.NoError(t, err)
	return &profilebootstrap.ResolvedOAuthProfile{Profile: oauthProfile, Store: store, Request: request}, store, request
}

func TestLoginCommandIsGlazedAndRegistersItsDeclaredFields(t *testing.T) {
	command, err := NewLoginCommand()
	require.NoError(t, err)
	var _ cmds.GlazeCommand = command

	root, err := NewAuthCommand()
	require.NoError(t, err)
	login, _, err := root.Find([]string{"login"})
	require.NoError(t, err)
	for _, name := range []string{"timeout-seconds", "open-browser", "profile", "profile-registries"} {
		require.NotNil(t, login.Flags().Lookup(name))
	}
}
