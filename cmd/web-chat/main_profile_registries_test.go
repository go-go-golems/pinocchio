package main

import (
	"os"
	"path/filepath"
	"testing"

	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func testValuesWithProfileRegistries(t *testing.T, profileRegistries string) *values.Values {
	t.Helper()

	profileSettingsSection, err := geppettosections.NewProfileSettingsSection()
	require.NoError(t, err)
	sectionValues, err := values.NewSectionValues(profileSettingsSection)
	require.NoError(t, err)
	sectionValues.Fields.Update("profile-registries", &fields.FieldValue{
		Value: []string{profileRegistries},
	})

	return values.New(values.WithSectionValues(geppettosections.ProfileSettingsSectionSlug, sectionValues))
}

func testValuesWithConfigFile(t *testing.T, configFile string) *values.Values {
	t.Helper()

	commandSection, err := cli.NewCommandSettingsSection()
	require.NoError(t, err)
	sectionValues, err := values.NewSectionValues(commandSection)
	require.NoError(t, err)
	sectionValues.Fields.Update("config-file", &fields.FieldValue{
		Value: configFile,
	})

	return values.New(values.WithSectionValues(cli.CommandSettingsSlug, sectionValues))
}

func TestResolveProfileRegistries_FallsBackToProfileSettingsSection(t *testing.T) {
	parsed := testValuesWithProfileRegistries(t, "./profiles.yaml")

	got := resolveProfileRegistries(parsed, nil)
	require.Equal(t, []string{"./profiles.yaml"}, got)

	gotValue, gotSource := resolveProfileRegistriesWithSource(parsed, nil)
	require.Equal(t, []string{"./profiles.yaml"}, gotValue)
	require.Equal(t, geppettosections.ProfileSettingsSectionSlug, gotSource)
}

func TestResolveProfileRegistries_PrefersDefaultSectionValue(t *testing.T) {
	parsed := testValuesWithProfileRegistries(t, "./profiles-from-profile-settings.yaml")

	got := resolveProfileRegistries(parsed, []string{"./profiles-from-default.yaml"})
	require.Equal(t, []string{"./profiles-from-default.yaml"}, got)

	gotValue, gotSource := resolveProfileRegistriesWithSource(parsed, []string{"./profiles-from-default.yaml"})
	require.Equal(t, []string{"./profiles-from-default.yaml"}, gotValue)
	require.Equal(t, "default-section", gotSource)
}

func TestResolveProfileRegistries_FallsBackToDefaultXDGProfilesPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("HOME", tmpDir)

	profilesDir := filepath.Join(tmpDir, "pinocchio")
	require.NoError(t, os.MkdirAll(profilesDir, 0o755))
	profilesPath := filepath.Join(profilesDir, "profiles.yaml")
	require.NoError(t, os.WriteFile(profilesPath, []byte("slug: default\nprofiles: {}\n"), 0o644))

	got := resolveProfileRegistries(values.New(), nil)
	require.Equal(t, []string{profilesPath}, got)

	gotValue, gotSource := resolveProfileRegistriesWithSource(values.New(), nil)
	require.Equal(t, []string{profilesPath}, gotValue)
	require.Equal(t, "xdg-default", gotSource)
}

func TestResolveWebChatConfigFiles_LoadsDefaultAndExplicitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	defaultDir := filepath.Join(tmpDir, ".pinocchio")
	require.NoError(t, os.MkdirAll(defaultDir, 0o755))
	defaultConfig := filepath.Join(defaultDir, "config.yaml")
	require.NoError(t, os.WriteFile(defaultConfig, []byte("{}\n"), 0o644))

	explicitConfig := filepath.Join(tmpDir, "override.yaml")
	require.NoError(t, os.WriteFile(explicitConfig, []byte("{}\n"), 0o644))

	got := resolveWebChatConfigFiles(testValuesWithConfigFile(t, explicitConfig))
	require.Equal(t, []string{defaultConfig, explicitConfig}, got)
}

func TestResolveWebChatBaseInferenceSettings_UsesDefaultsConfigAndEnv(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("PINOCCHIO_AI_ENGINE", "env-engine")

	defaultDir := filepath.Join(tmpDir, ".pinocchio")
	require.NoError(t, os.MkdirAll(defaultDir, 0o755))
	defaultConfig := filepath.Join(defaultDir, "config.yaml")
	require.NoError(t, os.WriteFile(defaultConfig, []byte(
		"ai-chat:\n  ai-engine: home-engine\nopenai-chat:\n  openai-api-key: home-key\n",
	), 0o644))

	explicitConfig := filepath.Join(tmpDir, "override.yaml")
	require.NoError(t, os.WriteFile(explicitConfig, []byte(
		"openai-chat:\n  openai-api-key: explicit-key\n",
	), 0o644))

	stepSettings, configFiles, err := resolveWebChatBaseInferenceSettings(testValuesWithConfigFile(t, explicitConfig))
	require.NoError(t, err)
	require.Equal(t, []string{defaultConfig, explicitConfig}, configFiles)
	require.NotNil(t, stepSettings.Chat.Engine)
	require.Equal(t, "env-engine", *stepSettings.Chat.Engine)
	require.Equal(t, "explicit-key", stepSettings.API.APIKeys["openai-api-key"])
	require.Equal(t, "https://api.openai.com/v1", stepSettings.API.BaseUrls["openai-base-url"])
}

func TestWebChatCommand_UsesPinocchioConfigNamespaceAndExposesOnlyProfileConfigFlags(t *testing.T) {
	cmdDef, err := NewCommand()
	require.NoError(t, err)

	cobraCmd, err := cli.BuildCobraCommand(cmdDef, cli.WithParserConfig(cli.CobraParserConfig{
		AppName: webChatCLIAppName,
		ConfigFilesFunc: func(_ *values.Values, _ *cobra.Command, _ []string) ([]string, error) {
			return nil, nil
		},
	}))
	require.NoError(t, err)

	for _, name := range []string{"print-yaml", "print-parsed-fields", "print-schema"} {
		flag := cobraCmd.Flags().Lookup(name)
		require.NotNil(t, flag)
		flag.Hidden = true
	}

	require.Equal(t, "pinocchio", webChatCLIAppName)
	require.Nil(t, cobraCmd.Flags().Lookup("ai-engine"))
	require.Nil(t, cobraCmd.Flags().Lookup("ai-api-type"))
	require.NotNil(t, cobraCmd.Flags().Lookup("profile-registries"))
	require.NotNil(t, cobraCmd.Flags().Lookup("profile"))
	configFlag := cobraCmd.Flags().Lookup("config-file")
	require.NotNil(t, configFlag)
	require.False(t, configFlag.Hidden)
	require.True(t, cobraCmd.Flags().Lookup("print-yaml").Hidden)
	require.True(t, cobraCmd.Flags().Lookup("print-parsed-fields").Hidden)
	require.True(t, cobraCmd.Flags().Lookup("print-schema").Hidden)
}
