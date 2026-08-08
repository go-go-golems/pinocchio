package profiles

import (
	"context"
	"encoding/json"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	openaisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	glazedconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/go-go-golems/glazed/pkg/types"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	"github.com/go-go-golems/pinocchio/pkg/configdoc"
	"github.com/go-go-golems/pinocchio/pkg/oauthprofiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebugCommandShowsStackAndRedactsCredentialValues(t *testing.T) {
	registryPath := writeRegistry(t, `slug: workspace
profiles:
  openai-base:
    slug: openai-base
    inference_settings:
      api:
        api_keys:
          openai-api-key: test-openai-key-must-not-appear
  embedding-small:
    slug: embedding-small
    stack:
      - profile_slug: openai-base
    inference_settings:
      embeddings:
        type: openai
        engine: text-embedding-3-small
        dimensions: 1536
  assistant:
    slug: assistant
    stack:
      - profile_slug: embedding-small
    inference_settings:
      chat:
        api_type: openai-responses
        engine: gpt-5
    extensions:
      pinocchio.oauth@v1:
        kind: oauth_bearer
        authorization_url: https://auth.example.com/authorize
        token_url: https://auth.example.com/token
        client_id: pinocchio-test
        access_token: oauth-access-token-must-not-appear
        refresh_token: ""
`)

	cmd, err := NewDebugCommand()
	require.NoError(t, err)
	processor := &captureProcessor{}
	require.NoError(t, cmd.RunIntoGlazeProcessor(context.Background(), parsedDebugValues(t, registryPath, "assistant", true), processor))

	baseLayer := findDebugRow(t, processor.rows, "profile-layer", "openai-base")
	assertStringSliceCellContains(t, baseLayer, "api_key_status", "openai-api-key=present")
	base := findDebugRow(t, processor.rows, "base", "")
	assertStringSliceCellContains(t, base, "api_key_status", "openai-api-key=present")
	final := findDebugRow(t, processor.rows, "final", "assistant")
	assertStringSliceCellContains(t, final, "api_key_status", "openai-api-key=present")
	assertStringSliceCellContains(t, final, "oauth_credential_status", "access_token=present")
	assertStringSliceCellContains(t, final, "oauth_credential_status", "refresh_token=empty")
	assertCell(t, final, "embedding_type", "openai")
	assertCell(t, final, "embedding_engine", "text-embedding-3-small")
	assertCell(t, final, "embedding_dimensions", 1536)

	encoded, err := json.Marshal(processor.rows)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "test-openai-key-must-not-appear", "debug output leaked a profile API key")
	assert.NotContains(t, string(encoded), "base-openai-key-must-not-appear", "debug output leaked a base API key")
	assert.NotContains(t, string(encoded), "oauth-access-token-must-not-appear", "debug output leaked an OAuth access token")
	assert.Contains(t, string(encoded), "***REDACTED***", "expected --include-settings to contain a redaction marker")
}

func TestAddOAuthCredentialStatusReportsPresenceWithoutValues(t *testing.T) {
	row := types.NewRow()
	addOAuthCredentialStatus(row, &oauthprofiles.Profile{Credential: credentials.Credential{
		AccessToken:  "access-token-must-not-appear",
		RefreshToken: "",
	}})

	status, ok := row.Get("oauth_credential_status")
	require.True(t, ok)
	assert.Equal(t, []string{"access_token=present", "refresh_token=empty"}, status)
	encoded, err := json.Marshal(row)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "access-token-must-not-appear")
}

func TestDebugProfileLayerRowIncludesLineageProvenance(t *testing.T) {
	row := debugProfileLayerRow(gepprofiles.ResolvedProfileStackEntry{
		RegistrySlug:      gepprofiles.MustRegistrySlug("workspace"),
		EngineProfileSlug: gepprofiles.MustEngineProfileSlug("assistant"),
		Source:            "/profiles/workspace.yaml",
		Version:           42,
	}, nil, false)

	assertCell(t, row, "source", "/profiles/workspace.yaml")
	assertCell(t, row, "version", uint64(42))
}

func TestDebugSourceRowsIncludeConfigDocuments(t *testing.T) {
	runtime := &profilebootstrap.ResolvedCLIProfileRuntime{
		Documents: &configdoc.ResolvedDocuments{Files: []glazedconfig.ResolvedConfigFile{{
			Path:       "/workspace/.pinocchio.yml",
			Layer:      glazedconfig.LayerCWD,
			SourceName: "cwd-local-profile",
		}}},
	}

	rows := debugSourceRows(runtime)
	require.Len(t, rows, 1)
	assertCell(t, rows[0], "source_kind", "config-document")
	assertCell(t, rows[0], "source_path", "/workspace/.pinocchio.yml")
	assertCell(t, rows[0], "source_name", "cwd-local-profile")
}

func parsedDebugValues(t *testing.T, registryPath, profile string, includeSettings bool) *values.Values {
	t.Helper()
	cmd, err := NewDebugCommand()
	require.NoError(t, err)
	parsed := values.New()
	defaultSection, ok := cmd.GetDefaultSection()
	require.True(t, ok, "missing default section")
	defaultValues, err := values.NewSectionValues(defaultSection)
	require.NoError(t, err)
	require.NoError(t, values.WithFieldValue("include-settings", includeSettings)(defaultValues))
	parsed.Set(values.DefaultSlug, defaultValues)

	profileSection, err := geppettosections.NewProfileSettingsSection()
	require.NoError(t, err)
	profileValues, err := values.NewSectionValues(profileSection)
	require.NoError(t, err)
	require.NoError(t, values.WithFieldValue("profile-registries", []string{registryPath})(profileValues))
	require.NoError(t, values.WithFieldValue("profile", profile)(profileValues))
	parsed.Set(geppettosections.ProfileSettingsSectionSlug, profileValues)

	openAISection, err := openaisettings.NewValueSection()
	require.NoError(t, err)
	openAIValues, err := values.NewSectionValues(openAISection)
	require.NoError(t, err)
	require.NoError(t, values.WithFieldValue("openai-api-key", "base-openai-key-must-not-appear")(openAIValues))
	parsed.Set(openaisettings.OpenAiChatSlug, openAIValues)
	return parsed
}

func findDebugRow(t *testing.T, rows []types.Row, stage, profile string) types.Row {
	t.Helper()
	for _, row := range rows {
		if cellString(t, row, "stage") == stage && cellString(t, row, "profile") == profile {
			return row
		}
	}
	require.FailNowf(t, "debug row not found", "stage=%q profile=%q rows=%#v", stage, profile, rows)
	return nil
}

func assertStringSliceCellContains(t *testing.T, row types.Row, key, want string) {
	t.Helper()
	got, ok := row.Get(types.FieldName(key))
	require.Truef(t, ok, "missing cell", "%q", key)
	values, ok := got.([]string)
	require.Truef(t, ok, "unexpected cell type", "%q: expected []string, got %T (%#v)", key, got, got)
	assert.Contains(t, values, want, "cell %q", key)
}
