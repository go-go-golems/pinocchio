package cmds

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
)

func TestLoadedCommandPrintProfilesUsesPinocchioInlineProfiles(t *testing.T) {
	tmpDir := isolateConfigDiscovery(t)
	configPath := filepath.Join(tmpDir, ".pinocchio.yml")
	configYAML := `profile:
  active: inline-prof
profiles:
  inline-prof:
    display_name: Inline Profile
    description: Inline profile from .pinocchio.yml
    inference_settings:
      chat:
        api_type: openai
        engine: inline-model
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmdYAML := `name: profile-introspection-smoke
short: smoke profile introspection
prompt: |
  say hello
`
	loaded, err := LoadFromYAML([]byte(cmdYAML))
	if err != nil {
		t.Fatalf("LoadFromYAML: %v", err)
	}
	cmd, ok := loaded[0].(*PinocchioCommand)
	if !ok {
		t.Fatalf("expected *PinocchioCommand, got %T", loaded[0])
	}
	recorder := &recordingEngineFactory{}
	cmd.EngineFactory = recorder

	parsed := values.New()
	helpersSection, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		t.Fatalf("helpers section: %v", err)
	}
	helpersValues, err := values.NewSectionValues(helpersSection)
	if err != nil {
		t.Fatalf("helpers values: %v", err)
	}
	parsed.Set(cmdlayers.GeppettoHelpersSlug, helpersValues)

	profileSection, err := geppettosections.NewProfileSettingsSection()
	if err != nil {
		t.Fatalf("profile section: %v", err)
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		t.Fatalf("profile values: %v", err)
	}
	parsed.Set(geppettosections.ProfileSettingsSectionSlug, profileValues)

	introspectionSection, err := geppettosections.NewProfileIntrospectionSection()
	if err != nil {
		t.Fatalf("profile introspection section: %v", err)
	}
	introspectionValues, err := values.NewSectionValues(introspectionSection)
	if err != nil {
		t.Fatalf("profile introspection values: %v", err)
	}
	if err := values.WithFieldValue("print-profiles", true)(introspectionValues); err != nil {
		t.Fatalf("set print-profiles: %v", err)
	}
	parsed.Set(geppettosections.ProfileIntrospectionSectionSlug, introspectionValues)

	var out bytes.Buffer
	if err := cmd.RunIntoWriter(context.Background(), parsed, &out); err != nil {
		t.Fatalf("RunIntoWriter: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "config-inline") {
		t.Fatalf("expected inline registry in output, got:\n%s", got)
	}
	if !strings.Contains(got, "inline-prof") {
		t.Fatalf("expected inline profile in output, got:\n%s", got)
	}
	if !strings.Contains(got, "inline-model") {
		t.Fatalf("expected inline model in output, got:\n%s", got)
	}
	if recorder.last != nil {
		t.Fatalf("expected print-profiles to exit before engine creation")
	}
}

func TestLoadedCommandPrintProfilesJSONIncludesResolution(t *testing.T) {
	tmpDir := isolateConfigDiscovery(t)
	registryPath := filepath.Join(tmpDir, "profiles.yaml")
	registryYAML := `slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai
        engine: base-model
  child:
    slug: child
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: child-model
`
	if err := os.WriteFile(registryPath, []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	cmdYAML := `name: profile-introspection-json
short: smoke profile introspection json
prompt: say hello
`
	loaded, err := LoadFromYAML([]byte(cmdYAML))
	if err != nil {
		t.Fatalf("LoadFromYAML: %v", err)
	}
	cmd := loaded[0].(*PinocchioCommand)

	parsed := values.New()
	helpersSection, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		t.Fatalf("helpers section: %v", err)
	}
	helpersValues, err := values.NewSectionValues(helpersSection)
	if err != nil {
		t.Fatalf("helpers values: %v", err)
	}
	parsed.Set(cmdlayers.GeppettoHelpersSlug, helpersValues)

	profileSection, err := geppettosections.NewProfileSettingsSection()
	if err != nil {
		t.Fatalf("profile section: %v", err)
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		t.Fatalf("profile values: %v", err)
	}
	if err := values.WithFieldValue("profile-registries", []string{registryPath})(profileValues); err != nil {
		t.Fatalf("set profile-registries: %v", err)
	}
	if err := values.WithFieldValue("profile", "child")(profileValues); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	parsed.Set(geppettosections.ProfileSettingsSectionSlug, profileValues)

	introspectionSection, err := geppettosections.NewProfileIntrospectionSection()
	if err != nil {
		t.Fatalf("profile introspection section: %v", err)
	}
	introspectionValues, err := values.NewSectionValues(introspectionSection)
	if err != nil {
		t.Fatalf("profile introspection values: %v", err)
	}
	for name, value := range map[string]any{
		"print-profiles":           true,
		"print-profile-resolution": true,
		"profile-output":           "json",
	} {
		if err := values.WithFieldValue(name, value)(introspectionValues); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	parsed.Set(geppettosections.ProfileIntrospectionSectionSlug, introspectionValues)

	var out bytes.Buffer
	if err := cmd.RunIntoWriter(context.Background(), parsed, &out); err != nil {
		t.Fatalf("RunIntoWriter: %v", err)
	}
	got := out.String()
	for _, needle := range []string{"\"selected_profile\": \"child\"", "\"resolution\"", "child-model", "base"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected %q in output, got:\n%s", needle, got)
		}
	}
}
