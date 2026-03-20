package cmds

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
)

type recordingEngineFactory struct {
	last *settings.InferenceSettings
}

func (f *recordingEngineFactory) CreateEngine(ss *settings.InferenceSettings) (engine.Engine, error) {
	if ss != nil {
		f.last = ss.Clone()
	}
	return recordingEngine{}, nil
}

func (f *recordingEngineFactory) SupportedProviders() []string {
	return []string{"openai"}
}

func (f *recordingEngineFactory) DefaultProvider() string {
	return "openai"
}

type recordingEngine struct{}

func (recordingEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	out := t.Clone()
	turns.AppendBlock(out, turns.NewAssistantTextBlock("ok"))
	return out, nil
}

func TestLoadedCommandRunIntoWriterUsesSelectedEngineProfile(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "profiles.yaml")
	registryYAML := `slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai
        engine: profiled-model
`
	if err := os.WriteFile(registryPath, []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	cmdYAML := `name: profile-registry-smoke
short: smoke loaded command
prompt: |
  say hello
`
	loaded, err := LoadFromYAML([]byte(cmdYAML))
	if err != nil {
		t.Fatalf("LoadFromYAML: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected one command, got %d", len(loaded))
	}

	cmd, ok := loaded[0].(*PinocchioCommand)
	if !ok {
		t.Fatalf("expected *PinocchioCommand, got %T", loaded[0])
	}
	recorder := &recordingEngineFactory{}
	cmd.EngineFactory = recorder

	parsed := values.New()

	defaultSection, err := schema.NewSection(values.DefaultSlug, "default")
	if err != nil {
		t.Fatalf("new default section: %v", err)
	}
	defaultValues, err := values.NewSectionValues(defaultSection)
	if err != nil {
		t.Fatalf("default section values: %v", err)
	}
	parsed.Set(values.DefaultSlug, defaultValues)

	sections, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		t.Fatalf("CreateGeppettoSections: %v", err)
	}
	var chatValues *values.SectionValues
	for _, section := range sections {
		sv, err := values.NewSectionValues(section)
		if err != nil {
			t.Fatalf("new section values for %s: %v", section.GetSlug(), err)
		}
		parsed.Set(section.GetSlug(), sv)
		if section.GetSlug() == settings.AiChatSlug {
			chatValues = sv
		}
	}
	if chatValues == nil {
		t.Fatalf("expected ai-chat section in geppetto sections")
	}
	if err := values.WithFieldValue("ai-api-type", "openai")(chatValues); err != nil {
		t.Fatalf("set ai-api-type: %v", err)
	}
	if err := values.WithFieldValue("ai-engine", "base-model")(chatValues); err != nil {
		t.Fatalf("set ai-engine: %v", err)
	}

	helpersSection, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		t.Fatalf("helpers section: %v", err)
	}
	helpersValues, err := values.NewSectionValues(helpersSection)
	if err != nil {
		t.Fatalf("helpers section values: %v", err)
	}
	parsed.Set(cmdlayers.GeppettoHelpersSlug, helpersValues)

	profileSection, err := geppettosections.NewProfileSettingsSection()
	if err != nil {
		t.Fatalf("profile section: %v", err)
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		t.Fatalf("profile section values: %v", err)
	}
	if err := values.WithFieldValue("profile-registries", []string{registryPath})(profileValues); err != nil {
		t.Fatalf("set profile-registries: %v", err)
	}
	if err := values.WithFieldValue("profile", "default")(profileValues); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	parsed.Set(geppettosections.ProfileSettingsSectionSlug, profileValues)

	var out bytes.Buffer
	if err := cmd.RunIntoWriter(context.Background(), parsed, &out); err != nil {
		t.Fatalf("RunIntoWriter: %v", err)
	}

	if recorder.last == nil || recorder.last.Chat == nil || recorder.last.Chat.Engine == nil {
		t.Fatalf("expected engine factory to receive inference settings with engine")
	}
	if got := strings.TrimSpace(*recorder.last.Chat.Engine); got != "profiled-model" {
		t.Fatalf("expected profiled-model, got %q", got)
	}
}

func TestLoadedCommandRunIntoWriterUsesLoaderBaselineInferenceSettings(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "profiles.yaml")
	registryYAML := `slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        engine: profiled-model
`
	if err := os.WriteFile(registryPath, []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	cmdYAML := `name: profile-registry-smoke
short: smoke loaded command
factories:
  chat:
    api_type: openai
    engine: loader-base-model
prompt: |
  say hello
`
	loaded, err := LoadFromYAML([]byte(cmdYAML))
	if err != nil {
		t.Fatalf("LoadFromYAML: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected one command, got %d", len(loaded))
	}

	cmd, ok := loaded[0].(*PinocchioCommand)
	if !ok {
		t.Fatalf("expected *PinocchioCommand, got %T", loaded[0])
	}
	recorder := &recordingEngineFactory{}
	cmd.EngineFactory = recorder

	parsed := values.New()

	defaultSection, err := schema.NewSection(values.DefaultSlug, "default")
	if err != nil {
		t.Fatalf("new default section: %v", err)
	}
	defaultValues, err := values.NewSectionValues(defaultSection)
	if err != nil {
		t.Fatalf("default section values: %v", err)
	}
	parsed.Set(values.DefaultSlug, defaultValues)

	helpersSection, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		t.Fatalf("helpers section: %v", err)
	}
	helpersValues, err := values.NewSectionValues(helpersSection)
	if err != nil {
		t.Fatalf("helpers section values: %v", err)
	}
	parsed.Set(cmdlayers.GeppettoHelpersSlug, helpersValues)

	sections, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		t.Fatalf("CreateGeppettoSections: %v", err)
	}
	for _, section := range sections {
		sv, err := values.NewSectionValues(section)
		if err != nil {
			t.Fatalf("new section values for %s: %v", section.GetSlug(), err)
		}
		parsed.Set(section.GetSlug(), sv)
		if section.GetSlug() == "openai-chat" {
			if err := values.WithFieldValue("openai-api-key", "config-key")(sv); err != nil {
				t.Fatalf("set openai-api-key: %v", err)
			}
		}
	}

	profileSection, err := geppettosections.NewProfileSettingsSection()
	if err != nil {
		t.Fatalf("profile section: %v", err)
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		t.Fatalf("profile section values: %v", err)
	}
	if err := values.WithFieldValue("profile-registries", []string{registryPath})(profileValues); err != nil {
		t.Fatalf("set profile-registries: %v", err)
	}
	if err := values.WithFieldValue("profile", "default")(profileValues); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	parsed.Set(geppettosections.ProfileSettingsSectionSlug, profileValues)

	var out bytes.Buffer
	if err := cmd.RunIntoWriter(context.Background(), parsed, &out); err != nil {
		t.Fatalf("RunIntoWriter: %v", err)
	}

	if recorder.last == nil || recorder.last.Chat == nil || recorder.last.Chat.Engine == nil || recorder.last.Chat.ApiType == nil {
		t.Fatalf("expected engine factory to receive full inference settings from loader baseline")
	}
	if got := strings.TrimSpace(*recorder.last.Chat.Engine); got != "profiled-model" {
		t.Fatalf("expected profiled-model, got %q", got)
	}
	if got := strings.TrimSpace(string(*recorder.last.Chat.ApiType)); got != "openai" {
		t.Fatalf("expected openai api type from loader baseline, got %q", got)
	}
	if got := strings.TrimSpace(recorder.last.API.APIKeys["openai-api-key"]); got != "config-key" {
		t.Fatalf("expected config openai api key to survive loader baseline merge, got %q", got)
	}
}

func TestLoadedCommandRunIntoWriterPrintsFinalInferenceSettings(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "profiles.yaml")
	registryYAML := `slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        engine: profiled-model
`
	if err := os.WriteFile(registryPath, []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	cmdYAML := `name: profile-registry-smoke
short: smoke loaded command
factories:
  chat:
    api_type: openai
    engine: loader-base-model
prompt: |
  say hello
`
	loaded, err := LoadFromYAML([]byte(cmdYAML))
	if err != nil {
		t.Fatalf("LoadFromYAML: %v", err)
	}
	cmd := loaded[0].(*PinocchioCommand)
	recorder := &recordingEngineFactory{}
	cmd.EngineFactory = recorder

	parsed := values.New()

	defaultSection, err := schema.NewSection(values.DefaultSlug, "default")
	if err != nil {
		t.Fatalf("new default section: %v", err)
	}
	defaultValues, err := values.NewSectionValues(defaultSection)
	if err != nil {
		t.Fatalf("default section values: %v", err)
	}
	parsed.Set(values.DefaultSlug, defaultValues)

	helpersSection, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		t.Fatalf("helpers section: %v", err)
	}
	helpersValues, err := values.NewSectionValues(helpersSection)
	if err != nil {
		t.Fatalf("helpers section values: %v", err)
	}
	if err := values.WithFieldValue("print-inference-settings", true)(helpersValues); err != nil {
		t.Fatalf("set print-inference-settings: %v", err)
	}
	parsed.Set(cmdlayers.GeppettoHelpersSlug, helpersValues)

	sections, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		t.Fatalf("CreateGeppettoSections: %v", err)
	}
	for _, section := range sections {
		sv, err := values.NewSectionValues(section)
		if err != nil {
			t.Fatalf("new section values for %s: %v", section.GetSlug(), err)
		}
		parsed.Set(section.GetSlug(), sv)
		if section.GetSlug() == "openai-chat" {
			if err := values.WithFieldValue("openai-api-key", "config-key")(sv); err != nil {
				t.Fatalf("set openai-api-key: %v", err)
			}
		}
	}

	profileSection, err := geppettosections.NewProfileSettingsSection()
	if err != nil {
		t.Fatalf("profile section: %v", err)
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		t.Fatalf("profile section values: %v", err)
	}
	if err := values.WithFieldValue("profile-registries", []string{registryPath})(profileValues); err != nil {
		t.Fatalf("set profile-registries: %v", err)
	}
	if err := values.WithFieldValue("profile", "default")(profileValues); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	parsed.Set(geppettosections.ProfileSettingsSectionSlug, profileValues)

	var out bytes.Buffer
	if err := cmd.RunIntoWriter(context.Background(), parsed, &out); err != nil {
		t.Fatalf("RunIntoWriter: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "settings:") || !strings.Contains(output, "sources:") {
		t.Fatalf("expected combined debug output, got %q", output)
	}
	if !strings.Contains(output, "engine: profiled-model") {
		t.Fatalf("expected profiled engine in printed settings, got %q", output)
	}
	if !strings.Contains(output, "openai-api-key: '***'") && !strings.Contains(output, "openai-api-key: \"***\"") && !strings.Contains(output, "openai-api-key: ***") {
		t.Fatalf("expected masked api key in printed settings, got %q", output)
	}
	if !strings.Contains(output, "source: config") {
		t.Fatalf("expected config source trace for api key, got %q", output)
	}
	if recorder.last != nil {
		t.Fatalf("expected print-inference-settings to exit before engine creation")
	}
}

func TestLoadedCommandRunIntoWriterPrintsInferenceSettingsWithSources(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "profiles.yaml")
	registryYAML := `slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        engine: profiled-model
`
	if err := os.WriteFile(registryPath, []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	cmdYAML := `name: profile-registry-smoke
short: smoke loaded command
factories:
  chat:
    api_type: openai
    engine: loader-base-model
prompt: |
  say hello
`
	loaded, err := LoadFromYAML([]byte(cmdYAML))
	if err != nil {
		t.Fatalf("LoadFromYAML: %v", err)
	}
	cmd := loaded[0].(*PinocchioCommand)
	recorder := &recordingEngineFactory{}
	cmd.EngineFactory = recorder

	parsed := values.New()

	defaultSection, err := schema.NewSection(values.DefaultSlug, "default")
	if err != nil {
		t.Fatalf("new default section: %v", err)
	}
	defaultValues, err := values.NewSectionValues(defaultSection)
	if err != nil {
		t.Fatalf("default section values: %v", err)
	}
	parsed.Set(values.DefaultSlug, defaultValues)

	helpersSection, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		t.Fatalf("helpers section: %v", err)
	}
	helpersValues, err := values.NewSectionValues(helpersSection)
	if err != nil {
		t.Fatalf("helpers section values: %v", err)
	}
	if err := values.WithFieldValue("print-inference-settings", true)(helpersValues); err != nil {
		t.Fatalf("set print-inference-settings: %v", err)
	}
	parsed.Set(cmdlayers.GeppettoHelpersSlug, helpersValues)

	sections, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		t.Fatalf("CreateGeppettoSections: %v", err)
	}
	for _, section := range sections {
		sv, err := values.NewSectionValues(section)
		if err != nil {
			t.Fatalf("new section values for %s: %v", section.GetSlug(), err)
		}
		parsed.Set(section.GetSlug(), sv)
		if section.GetSlug() == "openai-chat" {
			if err := values.WithFieldValue(
				"openai-api-key",
				"config-key",
				fields.WithSource("config"),
				fields.WithMetadata(map[string]any{"config_file": "/tmp/config.yaml"}),
			)(sv); err != nil {
				t.Fatalf("set openai-api-key: %v", err)
			}
		}
	}

	profileSection, err := geppettosections.NewProfileSettingsSection()
	if err != nil {
		t.Fatalf("profile section: %v", err)
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		t.Fatalf("profile section values: %v", err)
	}
	if err := values.WithFieldValue("profile-registries", []string{registryPath})(profileValues); err != nil {
		t.Fatalf("set profile-registries: %v", err)
	}
	if err := values.WithFieldValue("profile", "default")(profileValues); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	parsed.Set(geppettosections.ProfileSettingsSectionSlug, profileValues)

	var out bytes.Buffer
	if err := cmd.RunIntoWriter(context.Background(), parsed, &out); err != nil {
		t.Fatalf("RunIntoWriter: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "engine:") || !strings.Contains(output, "value: profiled-model") {
		t.Fatalf("expected profiled engine source trace, got %q", output)
	}
	if !strings.Contains(output, "source: command") {
		t.Fatalf("expected command baseline in trace, got %q", output)
	}
	if !strings.Contains(output, "source: profile") {
		t.Fatalf("expected profile source in trace, got %q", output)
	}
	if !strings.Contains(output, "openai-api-key:") || !strings.Contains(output, "source: config") {
		t.Fatalf("expected config source trace for api key, got %q", output)
	}
	if recorder.last != nil {
		t.Fatalf("expected print-inference-settings to exit before engine creation")
	}
}
