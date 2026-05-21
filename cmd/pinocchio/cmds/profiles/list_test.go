package profiles

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
)

var _ middlewares.Processor = (*captureProcessor)(nil)

type captureProcessor struct {
	rows []types.Row
}

func (p *captureProcessor) AddRow(ctx context.Context, row types.Row) error {
	p.rows = append(p.rows, row)
	return nil
}

func (p *captureProcessor) Close(ctx context.Context) error { return nil }

func TestListCommandShowsOverrideAndEffectiveSettings(t *testing.T) {
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
    display_name: Mini
    description: Cheap profile
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5-mini
      inference:
        reasoning_effort: low
        reasoning_summary: auto
`)

	rows := runListCommand(t, VerbosityDetailed, registryPath, "mini")
	row := findRow(t, rows, "workspace", "mini")

	assertCell(t, row, "selected", true)
	assertCell(t, row, "override_chat_engine", "gpt-5-mini")
	assertCell(t, row, "override_inference_reasoning_effort", "low")
	assertCell(t, row, "override_inference_reasoning_summary", "auto")
	assertCell(t, row, "effective_chat_engine", "gpt-5-mini")
	assertCell(t, row, "effective_chat_api_type", "openai-responses")
	assertCell(t, row, "effective_reasoning_effort", "low")
	assertCell(t, row, "reasoning_effort", "low")
	paths := cellString(t, row, "override_paths")
	for _, needle := range []string{"chat.engine", "inference.reasoning_effort", "inference.reasoning_summary"} {
		if !strings.Contains(paths, needle) {
			t.Fatalf("expected override_paths to contain %q, got %q", needle, paths)
		}
	}
}

func TestListCommandFullIncludesSettingsJSON(t *testing.T) {
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

	rows := runListCommand(t, VerbosityFull, registryPath, "mini")
	row := findRow(t, rows, "workspace", "mini")
	overrides := cellString(t, row, "override_settings_json")
	if !strings.Contains(overrides, "gpt-5-mini") || !strings.Contains(overrides, "reasoning_effort") {
		t.Fatalf("expected override_settings_json to include profile override settings, got %q", overrides)
	}
	effective := cellString(t, row, "effective_settings_json")
	if !strings.Contains(effective, "openai-responses") || !strings.Contains(effective, "gpt-5-mini") {
		t.Fatalf("expected effective_settings_json to include inherited and override settings, got %q", effective)
	}
}

func TestListCommandRejectsInvalidVerbosity(t *testing.T) {
	registryPath := writeRegistry(t, `slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        engine: gpt-5
`)

	cmd, err := NewListCommand()
	if err != nil {
		t.Fatalf("NewListCommand: %v", err)
	}
	parsed := parsedValues(t, "noisy", registryPath, "")
	processor := &captureProcessor{}
	err = cmd.RunIntoGlazeProcessor(context.Background(), parsed, processor)
	if err == nil || !strings.Contains(err.Error(), "invalid verbosity") {
		t.Fatalf("expected invalid verbosity error, got %v", err)
	}
}

func runListCommand(t *testing.T, verbosity string, registryPath string, profile string) []types.Row {
	t.Helper()
	cmd, err := NewListCommand()
	if err != nil {
		t.Fatalf("NewListCommand: %v", err)
	}
	parsed := parsedValues(t, verbosity, registryPath, profile)
	processor := &captureProcessor{}
	if err := cmd.RunIntoGlazeProcessor(context.Background(), parsed, processor); err != nil {
		t.Fatalf("RunIntoGlazeProcessor: %v", err)
	}
	return processor.rows
}

func parsedValues(t *testing.T, verbosity string, registryPath string, profile string) *values.Values {
	t.Helper()
	cmd, err := NewListCommand()
	if err != nil {
		t.Fatalf("NewListCommand: %v", err)
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
	if err := values.WithFieldValue("verbosity", verbosity)(defaultValues); err != nil {
		t.Fatalf("set verbosity: %v", err)
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
	if profile != "" {
		if err := values.WithFieldValue("profile", profile)(profileValues); err != nil {
			t.Fatalf("set profile: %v", err)
		}
	}
	parsed.Set(geppettosections.ProfileSettingsSectionSlug, profileValues)
	return parsed
}

func writeRegistry(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	return path
}

func findRow(t *testing.T, rows []types.Row, registry string, profile string) types.Row {
	t.Helper()
	for _, row := range rows {
		if cellString(t, row, "registry") == registry && cellString(t, row, "profile") == profile {
			return row
		}
	}
	t.Fatalf("row %s/%s not found in %d rows", registry, profile, len(rows))
	return nil
}

func assertCell(t *testing.T, row types.Row, key string, want any) {
	t.Helper()
	got, ok := row.Get(types.FieldName(key))
	if !ok {
		t.Fatalf("missing cell %q", key)
	}
	if got != want {
		t.Fatalf("cell %q: expected %#v, got %#v", key, want, got)
	}
}

func cellString(t *testing.T, row types.Row, key string) string {
	t.Helper()
	got, ok := row.Get(types.FieldName(key))
	if !ok || got == nil {
		return ""
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("cell %q: expected string, got %T (%#v)", key, got, got)
	}
	return s
}
