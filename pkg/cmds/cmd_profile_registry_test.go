package cmds

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
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

	profileSection, err := schema.NewSection(
		"profile-settings",
		"Profile settings",
		schema.WithFields(
			fields.New("profile", fields.TypeString),
			fields.New("profile-registries", fields.TypeString),
		),
	)
	if err != nil {
		t.Fatalf("profile section: %v", err)
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		t.Fatalf("profile section values: %v", err)
	}
	if err := values.WithFieldValue("profile-registries", registryPath)(profileValues); err != nil {
		t.Fatalf("set profile-registries: %v", err)
	}
	if err := values.WithFieldValue("profile", "default")(profileValues); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	parsed.Set("profile-settings", profileValues)

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
