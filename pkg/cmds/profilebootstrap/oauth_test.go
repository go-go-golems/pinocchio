//go:build !windows

package profilebootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/types"
	"github.com/go-go-golems/pinocchio/pkg/oauthprofiles"
	"github.com/stretchr/testify/require"
)

func TestResolveOAuthProfileFromOneDirectYAMLRegistry(t *testing.T) {
	resolved := oauthResolvedFixture(t, "")

	oauthProfile, err := ResolveOAuthProfile(context.Background(), resolved)
	require.NoError(t, err)
	require.NotNil(t, oauthProfile)
	require.Equal(t, credentials.Request{Provider: "openai", BaseURL: "https://provider.example.test/v1"}, oauthProfile.Request)
	require.Equal(t, "access-for-test", oauthProfile.Profile.Credential.AccessToken)

	source, err := oauthProfile.NewBearerTokenSource()
	require.NoError(t, err)
	token, err := source.BearerToken(context.Background(), oauthProfile.Request)
	require.NoError(t, err)
	require.Equal(t, "access-for-test", token)
}

func TestOAuthFactoryAcceptsSourceWithoutStaticKey(t *testing.T) {
	resolved := oauthResolvedFixture(t, "")
	engineFactory, err := NewEngineFactoryForResolvedSettings(context.Background(), resolved)
	require.NoError(t, err)
	_, err = engineFactory.CreateEngine(resolved.FinalInferenceSettings)
	require.NoError(t, err)
}

func TestResolveOAuthProfileRejectsStaticKey(t *testing.T) {
	resolved := oauthResolvedFixture(t, "static-key-must-not-appear")

	_, err := ResolveOAuthProfile(context.Background(), resolved)
	require.EqualError(t, err, "OAuth profile cannot also configure a static provider API key")
	require.NotContains(t, err.Error(), "static-key-must-not-appear")
}

func TestResolveOAuthProfileRejectsNonDirectSource(t *testing.T) {
	resolved := oauthResolvedFixture(t, "")
	resolved.ProfileRuntime.ProfileSettings.ProfileRegistries = nil

	_, err := ResolveOAuthProfile(context.Background(), resolved)
	require.EqualError(t, err, "OAuth profiles require an explicit direct YAML profile registry source")
}

func oauthResolvedFixture(t *testing.T, staticAPIKey string) *ResolvedCLIEngineSettings {
	t.Helper()
	registrySlug := gepprofiles.MustRegistrySlug("workspace")
	profileSlug := gepprofiles.MustEngineProfileSlug("assistant")
	profile := &gepprofiles.EngineProfile{
		Slug:       profileSlug,
		Extensions: testOAuthExtensions(),
	}
	registry := &gepprofiles.EngineProfileRegistry{
		Slug:                     registrySlug,
		DefaultEngineProfileSlug: profileSlug,
		Profiles:                 map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{profileSlug: profile},
	}
	data, err := gepprofiles.EncodeEngineProfileYAMLSingleRegistry(registry)
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	path := filepath.Join(dir, "profiles.yaml")
	require.NoError(t, os.WriteFile(path, data, 0o600))

	store := gepprofiles.NewInMemoryEngineProfileStore()
	require.NoError(t, store.UpsertRegistry(context.Background(), registry, gepprofiles.SaveOptions{}))
	reader, err := gepprofiles.NewStoreRegistry(store, registrySlug)
	require.NoError(t, err)

	apiType := types.ApiTypeOpenAI
	engineName := "test-model"
	finalSettings := &settings.InferenceSettings{
		Chat: &settings.ChatSettings{ApiType: &apiType, Engine: &engineName},
		API: &settings.APISettings{
			APIKeys:  map[string]string{"openai-api-key": staticAPIKey},
			BaseUrls: map[string]string{"openai-base-url": "https://provider.example.test/v1"},
		},
	}
	return &ResolvedCLIEngineSettings{
		FinalInferenceSettings: finalSettings,
		ProfileRuntime: &ResolvedCLIProfileRuntime{
			ProfileSettings:      ProfileSettings{ProfileRegistries: []string{path}},
			ProfileRegistryChain: bootstrapChainForOAuthTest{reader}.Resolved(),
		},
		ResolvedEngineProfile: &gepprofiles.ResolvedEngineProfile{
			RegistrySlug:      registrySlug,
			EngineProfileSlug: profileSlug,
		},
	}
}

// bootstrapChainForOAuthTest keeps the test fixture concise without making the
// production resolver depend on a concrete registry implementation.
type bootstrapChainForOAuthTest struct {
	reader gepprofiles.Registry
}

func (f bootstrapChainForOAuthTest) Resolved() *bootstrap.ResolvedProfileRegistryChain {
	return &bootstrap.ResolvedProfileRegistryChain{Registry: f.reader, Reader: f.reader}
}

func testOAuthExtensions() map[string]any {
	return map[string]any{
		oauthprofiles.ExtensionKey: map[string]any{
			"kind":                 oauthprofiles.OAuthBearerKind,
			"authorization_url":    "https://issuer.example.test/authorize",
			"token_url":            "https://issuer.example.test/token",
			"client_id":            "public-client",
			"refresh_token_policy": "preserve_previous",
			"access_token":         "access-for-test",
			"refresh_token":        "refresh-for-test",
			"expires_at":           time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
}
