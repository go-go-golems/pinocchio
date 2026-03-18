package helpers

import (
	"strings"
	"testing"
)

func TestMigrateEngineProfilesYAML_ConvertsMixedRuntimeProfiles(t *testing.T) {
	input := `
slug: workspace
default_profile_slug: assistant
profiles:
  base:
    slug: base
    runtime:
      step_settings_patch:
        ai-chat:
          ai-api-type: openai
          ai-engine: gpt-4o-mini
  assistant:
    slug: assistant
    stack:
      - profile_slug: base
    runtime:
      system_prompt: You are helpful.
      middlewares:
        - name: trace
      tools:
        - web_search
      step_settings_patch:
        ai-chat:
          ai-engine: gpt-5-mini
        ai-inference:
          reasoning-effort: medium
`

	registry, warnings, format, err := MigrateEngineProfilesYAML([]byte(input), "")
	if err != nil {
		t.Fatalf("MigrateEngineProfilesYAML failed: %v", err)
	}
	if format != "mixed-runtime" {
		t.Fatalf("expected mixed-runtime format, got %q", format)
	}
	if registry == nil {
		t.Fatal("expected registry")
	}
	if got := registry.DefaultEngineProfileSlug.String(); got != "assistant" {
		t.Fatalf("expected default assistant, got %q", got)
	}
	profile := registry.Profiles["assistant"]
	if profile == nil || profile.InferenceSettings == nil || profile.InferenceSettings.Chat == nil || profile.InferenceSettings.Chat.Engine == nil {
		t.Fatalf("expected assistant profile inference settings, got %#v", profile)
	}
	if got := *profile.InferenceSettings.Chat.Engine; got != "gpt-5-mini" {
		t.Fatalf("expected gpt-5-mini, got %q", got)
	}
	if len(warnings) != 3 {
		t.Fatalf("expected 3 warnings for dropped runtime fields, got %#v", warnings)
	}
}

func TestMigrateEngineProfilesYAML_ConvertsLegacyProfileMap(t *testing.T) {
	input := `
default:
  ai-chat:
    ai-api-type: openai
    ai-engine: gpt-4o-mini
assistant:
  ai-chat:
    ai-engine: gpt-5-mini
  ai-inference:
    reasoning-effort: medium
`

	registry, warnings, format, err := MigrateEngineProfilesYAML([]byte(input), "workspace")
	if err != nil {
		t.Fatalf("MigrateEngineProfilesYAML failed: %v", err)
	}
	if format != "legacy-map" {
		t.Fatalf("expected legacy-map format, got %q", format)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings for legacy map, got %#v", warnings)
	}
	if got := registry.Slug.String(); got != "workspace" {
		t.Fatalf("expected registry slug workspace, got %q", got)
	}
	if got := registry.DefaultEngineProfileSlug.String(); got != "default" {
		t.Fatalf("expected default profile slug default, got %q", got)
	}
	if got := *registry.Profiles["assistant"].InferenceSettings.Chat.Engine; got != "gpt-5-mini" {
		t.Fatalf("expected assistant model gpt-5-mini, got %q", got)
	}
}

func TestMigrateEngineProfilesYAML_PassesThroughEngineProfiles(t *testing.T) {
	input := `
slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-4o-mini
`

	registry, warnings, format, err := MigrateEngineProfilesYAML([]byte(input), "")
	if err != nil {
		t.Fatalf("MigrateEngineProfilesYAML failed: %v", err)
	}
	if format != "engine-profiles" {
		t.Fatalf("expected engine-profiles format, got %q", format)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if got := registry.Profiles["default"].InferenceSettings.Chat.Engine; got == nil || *got != "gpt-4o-mini" {
		t.Fatalf("expected default engine gpt-4o-mini, got %#v", registry.Profiles["default"].InferenceSettings.Chat.Engine)
	}
}

func TestMigrateEngineProfilesYAML_RejectsBundleFormat(t *testing.T) {
	_, _, _, err := MigrateEngineProfilesYAML([]byte("registries: {}\n"), "")
	if err == nil || !strings.Contains(err.Error(), "single-registry") {
		t.Fatalf("expected single-registry error, got %v", err)
	}
}
