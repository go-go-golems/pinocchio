package main

import (
	"context"
	"strings"
	"testing"

	embeddingscfg "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/gemini"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

type runtimeComposerTestSection struct {
	slug string
}

func (s runtimeComposerTestSection) GetDefinitions() *fields.Definitions {
	return fields.NewDefinitions()
}
func (s runtimeComposerTestSection) GetName() string        { return s.slug }
func (s runtimeComposerTestSection) GetDescription() string { return "" }
func (s runtimeComposerTestSection) GetPrefix() string      { return "" }
func (s runtimeComposerTestSection) GetSlug() string        { return s.slug }

func minimalRuntimeComposerValues(t *testing.T) *values.Values {
	t.Helper()

	slugs := []string{
		settings.AiClientSlug,
		settings.AiChatSlug,
		openai.OpenAiChatSlug,
		claude.ClaudeChatSlug,
		gemini.GeminiChatSlug,
		embeddingscfg.EmbeddingsSlug,
		settings.AiInferenceSlug,
	}
	opts := make([]values.ValuesOption, 0, len(slugs))
	for _, slug := range slugs {
		sectionValues, err := values.NewSectionValues(runtimeComposerTestSection{slug: slug})
		if err != nil {
			t.Fatalf("new section values for %s: %v", slug, err)
		}
		if slug == openai.OpenAiChatSlug {
			sectionValues.Fields.Update("openai-api-key", &fields.FieldValue{Value: "test-api-key"})
		}
		opts = append(opts, values.WithSectionValues(slug, sectionValues))
	}
	return values.New(opts...)
}

func TestRuntimeFingerprint_DoesNotIncludeAPIKeys(t *testing.T) {
	ss, err := settings.NewStepSettings()
	if err != nil {
		t.Fatalf("NewStepSettings: %v", err)
	}
	ss.API.APIKeys["openai"] = "sk-this-should-not-appear"

	fp := runtimeFingerprint("default", "hi", nil, nil, ss)
	if strings.Contains(fp, "sk-this-should-not-appear") {
		t.Fatalf("fingerprint leaked api key: %q", fp)
	}
	if strings.Contains(fp, "\"api_keys\"") || strings.Contains(fp, "\"APIKeys\"") {
		t.Fatalf("fingerprint unexpectedly contains api key fields: %q", fp)
	}
}

func TestWebChatRuntimeComposer_RejectsInvalidOverrideTypes(t *testing.T) {
	composer := newWebChatRuntimeComposer(values.New(), map[string]infruntime.MiddlewareFactory{})

	_, err := composer.Compose(context.Background(), infruntime.RuntimeComposeRequest{
		ConvID:     "c1",
		RuntimeKey: "default",
		Overrides:  map[string]any{"middlewares": "bad"},
	})
	if err == nil {
		t.Fatalf("expected error for invalid middlewares override type")
	}
}

func TestWebChatRuntimeComposer_UsesResolvedRuntimeSpec(t *testing.T) {
	composer := newWebChatRuntimeComposer(minimalRuntimeComposerValues(t), map[string]infruntime.MiddlewareFactory{})

	res, err := composer.Compose(context.Background(), infruntime.RuntimeComposeRequest{
		ConvID:     "c1",
		RuntimeKey: "analyst",
		ResolvedRuntime: &gepprofiles.RuntimeSpec{
			SystemPrompt: "You are analyst",
			Tools:        []string{"calculator", "  "},
		},
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	if res.SeedSystemPrompt != "You are analyst" {
		t.Fatalf("unexpected seed prompt: %q", res.SeedSystemPrompt)
	}
	if len(res.AllowedTools) != 1 || res.AllowedTools[0] != "calculator" {
		t.Fatalf("unexpected tools: %#v", res.AllowedTools)
	}
}

func TestWebChatRuntimeComposer_OverridesResolvedRuntimeSpec(t *testing.T) {
	composer := newWebChatRuntimeComposer(minimalRuntimeComposerValues(t), map[string]infruntime.MiddlewareFactory{})

	res, err := composer.Compose(context.Background(), infruntime.RuntimeComposeRequest{
		ConvID:     "c1",
		RuntimeKey: "analyst",
		ResolvedRuntime: &gepprofiles.RuntimeSpec{
			SystemPrompt: "You are analyst",
		},
		Overrides: map[string]any{
			"system_prompt": "Override prompt",
			"tools":         []any{"tool-a"},
		},
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	if res.SeedSystemPrompt != "Override prompt" {
		t.Fatalf("override not applied, got: %q", res.SeedSystemPrompt)
	}
	if len(res.AllowedTools) != 1 || res.AllowedTools[0] != "tool-a" {
		t.Fatalf("unexpected tools: %#v", res.AllowedTools)
	}
}
