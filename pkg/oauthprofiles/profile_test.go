package oauthprofiles

import (
	"testing"
	"time"

	geppettoauth "github.com/go-go-golems/geppetto/pkg/steps/ai/credentials/oauth"
	"github.com/stretchr/testify/require"
)

func TestParseOAuthBearerProfileAndRedactCredentials(t *testing.T) {
	expiresAt := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	extensions := testExtensions("access-for-test", "refresh-for-test", expiresAt)

	profile, err := Parse(extensions)
	require.NoError(t, err)
	require.NotNil(t, profile)
	require.Equal(t, "https://issuer.example.test/authorize", profile.AuthorizationURL)
	require.Equal(t, "https://issuer.example.test/token", profile.TokenURL)
	require.Equal(t, []string{"inference", "offline_access"}, profile.Scopes)
	require.Equal(t, geppettoauth.PreservePreviousRefreshToken, profile.RefreshTokenPolicy)
	require.Equal(t, "access-for-test", profile.Credential.AccessToken)
	require.Equal(t, "refresh-for-test", profile.Credential.RefreshToken)
	require.Equal(t, expiresAt, profile.Credential.ExpiresAt)

	redacted := RedactedExtensions(extensions)
	raw := redacted[ExtensionKey].(map[string]any)
	require.Equal(t, "<redacted>", raw["access_token"])
	require.Equal(t, "<redacted>", raw["refresh_token"])
	original := extensions[ExtensionKey].(map[string]any)
	require.Equal(t, "access-for-test", original["access_token"])
	require.Equal(t, "refresh-for-test", original["refresh_token"])
}

func TestParseRejectsClientSecretAndReturnsNoSecretInError(t *testing.T) {
	extensions := testExtensions("access-for-test", "refresh-for-test", time.Time{})
	raw := extensions[ExtensionKey].(map[string]any)
	raw["client_secret"] = "must-not-appear"

	_, err := Parse(extensions)
	require.EqualError(t, err, "OAuth profile client_secret is not supported; use a public PKCE client")
	require.NotContains(t, err.Error(), "must-not-appear")
}

func TestParseRejectsMalformedPolicyAndURL(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"policy": func(raw map[string]any) { raw["refresh_token_policy"] = "rotate-sometimes" },
		"URL":    func(raw map[string]any) { raw["token_url"] = "not-a-url" },
	} {
		t.Run(name, func(t *testing.T) {
			extensions := testExtensions("", "", time.Time{})
			raw := extensions[ExtensionKey].(map[string]any)
			mutate(raw)
			_, err := Parse(extensions)
			require.Error(t, err)
		})
	}
}

func testExtensions(accessToken, refreshToken string, expiresAt time.Time) map[string]any {
	raw := map[string]any{
		"kind":                 OAuthBearerKind,
		"authorization_url":    "https://issuer.example.test/authorize",
		"token_url":            "https://issuer.example.test/token",
		"client_id":            "public-client",
		"scopes":               []any{"inference", "offline_access"},
		"refresh_token_policy": "preserve_previous",
	}
	if accessToken != "" {
		raw["access_token"] = accessToken
	}
	if refreshToken != "" {
		raw["refresh_token"] = refreshToken
	}
	if !expiresAt.IsZero() {
		raw["expires_at"] = expiresAt.Format(time.RFC3339)
	}
	return map[string]any{ExtensionKey: raw}
}
