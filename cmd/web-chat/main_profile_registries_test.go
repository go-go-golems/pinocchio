package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	"github.com/stretchr/testify/require"
)

func testValuesWithProfileSettings(t *testing.T, profile string, profileRegistries []string) *values.Values {
	t.Helper()

	profileSettingsSection, err := profilebootstrap.NewProfileSettingsSection()
	require.NoError(t, err)
	sectionValues, err := values.NewSectionValues(profileSettingsSection)
	require.NoError(t, err)
	if profile != "" {
		require.NoError(t, values.WithFieldValue("profile", profile)(sectionValues))
	}
	if len(profileRegistries) > 0 {
		require.NoError(t, values.WithFieldValue("profile-registries", profileRegistries)(sectionValues))
	}

	return values.New(values.WithSectionValues(profilebootstrap.ProfileSettingsSectionSlug, sectionValues))
}

func testValuesWithConfigFile(t *testing.T, configFile string) *values.Values {
	t.Helper()

	commandSection, err := cli.NewCommandSettingsSection()
	require.NoError(t, err)
	sectionValues, err := values.NewSectionValues(commandSection)
	require.NoError(t, err)
	require.NoError(t, values.WithFieldValue("config-file", configFile, fields.WithSource("cli"))(sectionValues))

	return values.New(values.WithSectionValues(cli.CommandSettingsSlug, sectionValues))
}

func testValuesWithConfigFileClientSettings(t *testing.T, configFile string, timeout int, proxyURL string) *values.Values {
	t.Helper()

	parsed := testValuesWithConfigFile(t, configFile)
	clientSection, err := aisettings.NewClientValueSection()
	require.NoError(t, err)
	options := []values.SectionValuesOption{
		values.WithFieldValue("timeout", timeout, fields.WithSource("cobra")),
	}
	if proxyURL != "" {
		options = append(options, values.WithFieldValue("proxy-url", proxyURL, fields.WithSource("cobra")))
	}
	clientValues, err := values.NewSectionValues(clientSection, options...)
	require.NoError(t, err)
	parsed.Set(aisettings.AiClientSlug, clientValues)
	return parsed
}

func TestWebChatProfileSelection_UsesSharedProfileSettingsSection(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	parsed := testValuesWithProfileSettings(t, "analyst", []string{"./profiles.yaml"})

	resolved, err := profilebootstrap.ResolveCLIProfileSelection(parsed)
	require.NoError(t, err)
	require.Equal(t, "analyst", resolved.Profile)
	require.Equal(t, []string{"./profiles.yaml"}, resolved.ProfileRegistries)
}

func TestWebChatProfileSelection_DoesNotFallbackToDefaultRegistryFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("HOME", tmpDir)

	profilesDir := filepath.Join(tmpDir, "pinocchio")
	require.NoError(t, os.MkdirAll(profilesDir, 0o755))
	profilesPath := filepath.Join(profilesDir, "profiles.yaml")
	require.NoError(t, os.WriteFile(profilesPath, []byte("slug: default\nprofiles: {}\n"), 0o644))

	resolved, err := profilebootstrap.ResolveCLIProfileSelection(values.New())
	require.NoError(t, err)
	require.Empty(t, resolved.ProfileRegistries)
}

func TestWebChatUnifiedProfileConfig_AllowsInlineProfilesWithoutExternalRegistries(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
	cwd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(cwd) }()
	require.NoError(t, os.Chdir(tmpDir))

	configPath := filepath.Join(tmpDir, ".pinocchio.yml")
	require.NoError(t, os.WriteFile(configPath, []byte(
		"profile:\n  active: analyst\nprofiles:\n  analyst:\n    inference_settings:\n      chat:\n        api_type: openai-responses\n        engine: gpt-5-mini\n",
	), 0o644))

	resolvedConfig, err := profilebootstrap.ResolveUnifiedConfig(values.New())
	require.NoError(t, err)
	require.Equal(t, "analyst", resolvedConfig.ProfileSettings.Profile)
	require.Empty(t, resolvedConfig.ProfileSettings.ProfileRegistries)

	chain, err := profilebootstrap.ResolveUnifiedProfileRegistryChain(context.Background(), resolvedConfig)
	require.NoError(t, err)
	require.NotNil(t, chain)
	require.NotNil(t, chain.Registry)
	require.NotNil(t, chain.Reader)
	require.Equal(t, "config-inline", chain.DefaultRegistrySlug.String())
	if chain.Close != nil {
		defer chain.Close()
	}
}

func TestResolveBaseInferenceSettings_UsesEnvAndReturnsResolvedUnifiedConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("PINOCCHIO_AI_ENGINE", "env-engine")

	defaultDir := filepath.Join(tmpDir, ".pinocchio")
	require.NoError(t, os.MkdirAll(defaultDir, 0o755))
	defaultConfig := filepath.Join(defaultDir, "config.yaml")
	require.NoError(t, os.WriteFile(defaultConfig, []byte(
		"profile:\n  active: default\nprofiles:\n  default:\n    inference_settings:\n      chat:\n        api_type: openai-responses\n        engine: gpt-5-mini\n",
	), 0o644))

	explicitConfig := filepath.Join(tmpDir, "override.yaml")
	require.NoError(t, os.WriteFile(explicitConfig, []byte(
		"profile:\n  active: other\n",
	), 0o644))

	stepSettings, configFiles, err := profilebootstrap.ResolveBaseInferenceSettings(testValuesWithConfigFile(t, explicitConfig))
	require.NoError(t, err)
	require.Equal(t, []string{defaultConfig, explicitConfig}, configFiles)
	require.NotNil(t, stepSettings.Chat.Engine)
	require.Equal(t, "env-engine", *stepSettings.Chat.Engine)
	require.Equal(t, "https://api.openai.com/v1", stepSettings.API.BaseUrls["openai-base-url"])
}

func TestWebChatCommand_UsesPinocchioConfigNamespaceAndExposesProfileAndAIClientFlags(t *testing.T) {
	cmdDef, err := NewCommand()
	require.NoError(t, err)

	cobraCmd, err := cli.BuildCobraCommand(cmdDef, cli.WithParserConfig(cli.CobraParserConfig{
		AppName: webChatCLIAppName,
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
	require.NotNil(t, cobraCmd.Flags().Lookup("timeout"))
	require.NotNil(t, cobraCmd.Flags().Lookup("organization"))
	require.NotNil(t, cobraCmd.Flags().Lookup("user-agent"))
	require.NotNil(t, cobraCmd.Flags().Lookup("proxy-url"))
	require.NotNil(t, cobraCmd.Flags().Lookup("proxy-from-environment"))
	require.NotNil(t, cobraCmd.Flags().Lookup("profile-registries"))
	require.NotNil(t, cobraCmd.Flags().Lookup("profile"))
	configFlag := cobraCmd.Flags().Lookup("config-file")
	require.NotNil(t, configFlag)
	require.False(t, configFlag.Hidden)
	require.True(t, cobraCmd.Flags().Lookup("print-yaml").Hidden)
	require.True(t, cobraCmd.Flags().Lookup("print-parsed-fields").Hidden)
	require.True(t, cobraCmd.Flags().Lookup("print-schema").Hidden)
}

func TestResolveParsedBaseInferenceSettingsWithBase_AppliesWebChatClientCLIFlags(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	defaultDir := filepath.Join(tmpDir, ".pinocchio")
	require.NoError(t, os.MkdirAll(defaultDir, 0o755))
	defaultConfig := filepath.Join(defaultDir, "config.yaml")
	require.NoError(t, os.WriteFile(defaultConfig, []byte(
		"profile:\n  active: default\n",
	), 0o644))

	parsed := testValuesWithConfigFileClientSettings(t, defaultConfig, 123, "http://cli-proxy.internal:8080")

	hiddenBase, _, err := profilebootstrap.ResolveBaseInferenceSettings(parsed)
	require.NoError(t, err)
	require.NotNil(t, hiddenBase.Client)
	if hiddenBase.Client.TimeoutSeconds != nil {
		require.NotEqual(t, 123, *hiddenBase.Client.TimeoutSeconds)
	}
	if hiddenBase.Client.ProxyURL != nil {
		require.NotEqual(t, "http://cli-proxy.internal:8080", *hiddenBase.Client.ProxyURL)
	}

	baseWithCLI, err := profilebootstrap.ResolveParsedBaseInferenceSettingsWithBase(parsed, hiddenBase)
	require.NoError(t, err)
	require.NotNil(t, baseWithCLI.Client)
	require.NotNil(t, baseWithCLI.Client.TimeoutSeconds)
	require.Equal(t, 123, *baseWithCLI.Client.TimeoutSeconds)
	require.NotNil(t, baseWithCLI.Client.ProxyURL)
	require.Equal(t, "http://cli-proxy.internal:8080", *baseWithCLI.Client.ProxyURL)
}
