//go:build windows

package oauthprofiles

import (
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	"github.com/stretchr/testify/require"
)

func TestNewYAMLStoreRejectsUnsupportedWindowsPersistence(t *testing.T) {
	_, err := NewYAMLStore(
		`C:\private\profiles.yaml`,
		gepprofiles.MustRegistrySlug("workspace"),
		gepprofiles.MustEngineProfileSlug("assistant"),
		credentials.Request{Provider: "openai", BaseURL: "https://provider.example.test/v1"},
	)
	require.EqualError(t, err, "OAuth profile YAML persistence is not supported on Windows")
}
