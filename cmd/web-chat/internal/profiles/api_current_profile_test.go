package profiles

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/stretchr/testify/require"
)

func testProfileWithRuntime(t *testing.T, slug string) *gepprofiles.EngineProfile {
	t.Helper()
	profile := &gepprofiles.EngineProfile{Slug: gepprofiles.MustEngineProfileSlug(slug)}
	require.NoError(t, infruntime.SetProfileRuntime(profile, &infruntime.ProfileRuntime{SystemPrompt: "profile " + slug}))
	return profile
}

func newProfileAPITestServer(t *testing.T) *httptest.Server {
	t.Helper()
	registry, err := NewInMemoryProfileService("alpha", testProfileWithRuntime(t, "alpha"), testProfileWithRuntime(t, "beta"))
	require.NoError(t, err)
	mux := http.NewServeMux()
	RegisterAPIHandlers(mux, registry, APIOptions{
		DefaultRegistrySlug:             gepprofiles.MustRegistrySlug(DefaultRegistrySlug),
		EnableCurrentProfileCookieRoute: true,
		CurrentProfileCookieName:        "chat_profile",
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestCurrentProfileRouteDefaultsToRegistryDefault(t *testing.T) {
	server := newProfileAPITestServer(t)

	resp, err := http.Get(server.URL + "/api/chat/profile")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := readAllString(t, resp)
	require.Contains(t, body, `"profile":"alpha"`)
	require.Contains(t, body, `"registry":"default"`)
}

func TestCurrentProfileRouteAcceptsQualifiedAndLegacyCookieValues(t *testing.T) {
	server := newProfileAPITestServer(t)

	for _, cookieValue := range []string{"default/beta", "beta"} {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/chat/profile", nil)
		require.NoError(t, err)
		req.AddCookie(&http.Cookie{Name: "chat_profile", Value: cookieValue})
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		body := readAllString(t, resp)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Contains(t, body, `"profile":"beta"`)
		require.Contains(t, body, `"registry":"default"`)
	}
}

func TestCurrentProfileRoutePostSetsQualifiedSecureCookie(t *testing.T) {
	server := newProfileAPITestServer(t)

	resp, err := http.Post(server.URL+"/api/chat/profile", "application/json", strings.NewReader(`{"profile":"beta","registry":"default"}`))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	require.Equal(t, "chat_profile", cookie.Name)
	require.Equal(t, "default/beta", cookie.Value)
	require.Equal(t, "/", cookie.Path)
	require.True(t, cookie.Secure)
	require.True(t, cookie.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func readAllString(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}
