package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/stretchr/testify/require"
)

type webChatResolverTestSection struct {
	slug string
}

func (s webChatResolverTestSection) GetDefinitions() *fields.Definitions {
	return fields.NewDefinitions()
}
func (s webChatResolverTestSection) GetName() string        { return s.slug }
func (s webChatResolverTestSection) GetDescription() string { return "" }
func (s webChatResolverTestSection) GetPrefix() string      { return "" }
func (s webChatResolverTestSection) GetSlug() string        { return s.slug }

func testValuesWithProfileRegistries(t *testing.T, profileRegistries string) *values.Values {
	t.Helper()

	sectionValues, err := values.NewSectionValues(webChatResolverTestSection{
		slug: webChatProfileSettingsSectionSlug,
	})
	require.NoError(t, err)
	sectionValues.Fields.Update("profile-registries", &fields.FieldValue{
		Value: profileRegistries,
	})

	return values.New(values.WithSectionValues(webChatProfileSettingsSectionSlug, sectionValues))
}

func TestResolveProfileRegistries_FallsBackToProfileSettingsSection(t *testing.T) {
	parsed := testValuesWithProfileRegistries(t, "./profiles.yaml")

	got := resolveProfileRegistries(parsed, "")
	require.Equal(t, "./profiles.yaml", got)
}

func TestResolveProfileRegistries_PrefersDefaultSectionValue(t *testing.T) {
	parsed := testValuesWithProfileRegistries(t, "./profiles-from-profile-settings.yaml")

	got := resolveProfileRegistries(parsed, "./profiles-from-default.yaml")
	require.Equal(t, "./profiles-from-default.yaml", got)
}

func TestResolveProfileRegistries_FallsBackToDefaultXDGProfilesPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("HOME", tmpDir)

	profilesDir := filepath.Join(tmpDir, "pinocchio")
	require.NoError(t, os.MkdirAll(profilesDir, 0o755))
	profilesPath := filepath.Join(profilesDir, "profiles.yaml")
	require.NoError(t, os.WriteFile(profilesPath, []byte("slug: default\nprofiles: {}\n"), 0o644))

	got := resolveProfileRegistries(values.New(), "")
	require.Equal(t, profilesPath, got)
}
