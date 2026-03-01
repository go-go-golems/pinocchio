package main

import (
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
