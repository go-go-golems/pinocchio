package profiles

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/types"
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
`)

	cmd, err := NewDebugCommand()
	if err != nil {
		t.Fatalf("NewDebugCommand: %v", err)
	}
	processor := &captureProcessor{}
	if err := cmd.RunIntoGlazeProcessor(context.Background(), parsedDebugValues(t, registryPath, "assistant", true), processor); err != nil {
		t.Fatalf("RunIntoGlazeProcessor: %v", err)
	}

	baseLayer := findDebugRow(t, processor.rows, "profile-layer", "openai-base")
	assertStringSliceCellContains(t, baseLayer, "api_key_status", "openai-api-key=present")
	final := findDebugRow(t, processor.rows, "final", "assistant")
	assertStringSliceCellContains(t, final, "api_key_status", "openai-api-key=present")
	assertCell(t, final, "embedding_type", "openai")
	assertCell(t, final, "embedding_engine", "text-embedding-3-small")
	assertCell(t, final, "embedding_dimensions", 1536)

	encoded, err := json.Marshal(processor.rows)
	if err != nil {
		t.Fatalf("marshal debug output: %v", err)
	}
	if strings.Contains(string(encoded), "test-openai-key-must-not-appear") {
		t.Fatalf("debug output leaked an API key: %s", encoded)
	}
	if !strings.Contains(string(encoded), "***REDACTED***") {
		t.Fatalf("expected --include-settings to contain a redaction marker, got %s", encoded)
	}
}

func parsedDebugValues(t *testing.T, registryPath, profile string, includeSettings bool) *values.Values {
	t.Helper()
	cmd, err := NewDebugCommand()
	if err != nil {
		t.Fatalf("NewDebugCommand: %v", err)
	}
	parsed := values.New()
	defaultSection, ok := cmd.GetDefaultSection()
	if !ok {
		t.Fatal("missing default section")
	}
	defaultValues, err := values.NewSectionValues(defaultSection)
	if err != nil {
		t.Fatalf("default values: %v", err)
	}
	if err := values.WithFieldValue("include-settings", includeSettings)(defaultValues); err != nil {
		t.Fatalf("set include-settings: %v", err)
	}
	parsed.Set(values.DefaultSlug, defaultValues)

	profileSection, err := geppettosections.NewProfileSettingsSection()
	if err != nil {
		t.Fatalf("profile section: %v", err)
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		t.Fatalf("profile values: %v", err)
	}
	if err := values.WithFieldValue("profile-registries", []string{registryPath})(profileValues); err != nil {
		t.Fatalf("set profile registries: %v", err)
	}
	if err := values.WithFieldValue("profile", profile)(profileValues); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	parsed.Set(geppettosections.ProfileSettingsSectionSlug, profileValues)
	return parsed
}

func findDebugRow(t *testing.T, rows []types.Row, stage, profile string) types.Row {
	t.Helper()
	for _, row := range rows {
		if cellString(t, row, "stage") == stage && cellString(t, row, "profile") == profile {
			return row
		}
	}
	t.Fatalf("debug row stage=%q profile=%q not found in %#v", stage, profile, rows)
	return nil
}

func assertStringSliceCellContains(t *testing.T, row types.Row, key, want string) {
	t.Helper()
	got, ok := row.Get(types.FieldName(key))
	if !ok {
		t.Fatalf("missing cell %q", key)
	}
	values, ok := got.([]string)
	if !ok {
		t.Fatalf("cell %q: expected []string, got %T (%#v)", key, got, got)
	}
	if !containsString(values, want) {
		t.Fatalf("cell %q: expected %#v to contain %q", key, values, want)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
