package profiles

import (
	"context"
	"strings"
	"testing"

	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/types"
)

func TestShowCommandShowsRequestedProfile(t *testing.T) {
	registryPath := writeRegistry(t, `slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai-responses
        engine: gpt-5
      inference:
        reasoning_effort: medium
  mini:
    slug: mini
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5-mini
      inference:
        reasoning_effort: low
`)

	row := runShowCommand(t, VerbosityDetailed, registryPath, "", "workspace/mini", "")
	assertCell(t, row, "registry", "workspace")
	assertCell(t, row, "profile", "mini")
	assertCell(t, row, "override_chat_engine", "gpt-5-mini")
	assertCell(t, row, "override_inference_reasoning_effort", "low")
	assertCell(t, row, "effective_chat_api_type", "openai-responses")
	assertCell(t, row, "effective_reasoning_effort", "low")
}

func TestShowCommandDefaultsToSelectedProfile(t *testing.T) {
	registryPath := writeRegistry(t, `slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai-responses
        engine: gpt-5
  mini:
    slug: mini
    inference_settings:
      chat:
        engine: gpt-5-mini
`)

	row := runShowCommand(t, VerbosityDetailed, registryPath, "mini", "", "")
	assertCell(t, row, "registry", "workspace")
	assertCell(t, row, "profile", "mini")
	assertCell(t, row, "selected", true)
}

func TestShowCommandFullIncludesSettingsJSON(t *testing.T) {
	registryPath := writeRegistry(t, `slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai-responses
        engine: gpt-5
  mini:
    slug: mini
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5-mini
      inference:
        reasoning_effort: low
`)

	row := runShowCommand(t, VerbosityFull, registryPath, "", "mini", "workspace")
	overrides := cellString(t, row, "override_settings_json")
	if !strings.Contains(overrides, "gpt-5-mini") || !strings.Contains(overrides, "reasoning_effort") {
		t.Fatalf("expected override settings JSON to include raw override fields, got %q", overrides)
	}
	effective := cellString(t, row, "effective_settings_json")
	if !strings.Contains(effective, "openai-responses") || !strings.Contains(effective, "gpt-5-mini") {
		t.Fatalf("expected effective settings JSON to include inherited and raw fields, got %q", effective)
	}
}

func runShowCommand(t *testing.T, verbosity string, registryPath string, selectedProfile string, profileRef string, registry string) types.Row {
	t.Helper()
	cmd, err := NewShowCommand()
	if err != nil {
		t.Fatalf("NewShowCommand: %v", err)
	}
	parsed := parsedShowValues(t, verbosity, registryPath, selectedProfile, profileRef, registry)
	processor := &captureProcessor{}
	if err := cmd.RunIntoGlazeProcessor(context.Background(), parsed, processor); err != nil {
		t.Fatalf("RunIntoGlazeProcessor: %v", err)
	}
	if len(processor.rows) != 1 {
		t.Fatalf("expected one row, got %d", len(processor.rows))
	}
	return processor.rows[0]
}

func parsedShowValues(t *testing.T, verbosity string, registryPath string, selectedProfile string, profileRef string, registry string) *values.Values {
	t.Helper()
	cmd, err := NewShowCommand()
	if err != nil {
		t.Fatalf("NewShowCommand: %v", err)
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
	for name, value := range map[string]any{
		"verbosity":   verbosity,
		"profile-ref": profileRef,
		"registry":    registry,
	} {
		if err := values.WithFieldValue(name, value)(defaultValues); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
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
	if selectedProfile != "" {
		if err := values.WithFieldValue("profile", selectedProfile)(profileValues); err != nil {
			t.Fatalf("set profile: %v", err)
		}
	}
	parsed.Set(geppettosections.ProfileSettingsSectionSlug, profileValues)
	return parsed
}
