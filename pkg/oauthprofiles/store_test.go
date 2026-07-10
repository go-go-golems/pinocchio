package oauthprofiles

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	"github.com/stretchr/testify/require"
)

func TestYAMLStoreLoadsAndAtomicallyRotatesOneProfile(t *testing.T) {
	path := writeRegistryFixture(t, 0o600)
	store := newTestStore(t, path)
	request := testRequest()

	loaded, err := store.Load(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "old-access", loaded.AccessToken)
	require.Equal(t, "old-refresh", loaded.RefreshToken)

	replacement := credentials.Credential{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresAt:    time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	require.NoError(t, store.Save(context.Background(), request, replacement))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	rotated, err := store.Load(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, replacement, rotated)

	registry := readRegistry(t, path)
	other, err := Parse(registry.Profiles[gepprofiles.MustEngineProfileSlug("other")].Extensions)
	require.NoError(t, err)
	require.Equal(t, "other-access", other.Credential.AccessToken)
	require.Equal(t, "other-refresh", other.Credential.RefreshToken)
}

func TestYAMLStoreRejectsUnsafePermissionsAndMismatchedRequest(t *testing.T) {
	path := writeRegistryFixture(t, 0o644)
	store := newTestStore(t, path)

	_, err := store.Load(context.Background(), testRequest())
	require.EqualError(t, err, "OAuth profile registry must have mode 0600")

	require.NoError(t, os.Chmod(path, 0o600))
	mismatch := credentials.Request{Provider: "openai", BaseURL: "https://another.example.test/v1"}
	err = store.Save(context.Background(), mismatch, credentials.Credential{AccessToken: "new-access", RefreshToken: "new-refresh"})
	require.EqualError(t, err, "OAuth credential request does not match the selected profile")

	loaded, err := store.Load(context.Background(), testRequest())
	require.NoError(t, err)
	require.Equal(t, "old-access", loaded.AccessToken)
}

func TestYAMLStoreRequiresCompleteReplacementTuple(t *testing.T) {
	path := writeRegistryFixture(t, 0o600)
	store := newTestStore(t, path)

	err := store.Save(context.Background(), testRequest(), credentials.Credential{AccessToken: "new-access"})
	require.EqualError(t, err, "OAuth credential refresh token is required")
}

func newTestStore(t *testing.T, path string) *YAMLStore {
	t.Helper()
	store, err := NewYAMLStore(
		path,
		gepprofiles.MustRegistrySlug("workspace"),
		gepprofiles.MustEngineProfileSlug("assistant"),
		testRequest(),
	)
	require.NoError(t, err)
	return store
}

func testRequest() credentials.Request {
	return credentials.Request{Provider: "openai", BaseURL: "https://provider.example.test/v1/"}
}

func writeRegistryFixture(t *testing.T, mode os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	path := filepath.Join(dir, "profiles.yaml")
	registry := &gepprofiles.EngineProfileRegistry{
		Slug:                     gepprofiles.MustRegistrySlug("workspace"),
		DefaultEngineProfileSlug: gepprofiles.MustEngineProfileSlug("assistant"),
		Profiles: map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{
			gepprofiles.MustEngineProfileSlug("assistant"): {
				Slug:       gepprofiles.MustEngineProfileSlug("assistant"),
				Extensions: testExtensions("old-access", "old-refresh", time.Date(2029, 1, 2, 3, 4, 5, 0, time.UTC)),
			},
			gepprofiles.MustEngineProfileSlug("other"): {
				Slug:       gepprofiles.MustEngineProfileSlug("other"),
				Extensions: testExtensions("other-access", "other-refresh", time.Time{}),
			},
		},
	}
	data, err := gepprofiles.EncodeEngineProfileYAMLSingleRegistry(registry)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, mode))
	return path
}

func readRegistry(t *testing.T, path string) *gepprofiles.EngineProfileRegistry {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	registry, err := gepprofiles.DecodeEngineProfileYAMLSingleRegistry(data)
	require.NoError(t, err)
	require.NotNil(t, registry)
	return registry
}
