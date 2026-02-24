package main

import (
	"context"
	"strings"
	"testing"

	embeddingscfg "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	gepmiddleware "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
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

type runtimeComposerDefinition struct {
	name    string
	schema  map[string]any
	buildFn func(context.Context, middlewarecfg.BuildDeps, any) (gepmiddleware.Middleware, error)
}

func (d *runtimeComposerDefinition) Name() string {
	return d.name
}

func (d *runtimeComposerDefinition) ConfigJSONSchema() map[string]any {
	return d.schema
}

func (d *runtimeComposerDefinition) Build(ctx context.Context, deps middlewarecfg.BuildDeps, cfg any) (gepmiddleware.Middleware, error) {
	if d.buildFn == nil {
		return func(next gepmiddleware.HandlerFunc) gepmiddleware.HandlerFunc { return next }, nil
	}
	return d.buildFn(ctx, deps, cfg)
}

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

func newRuntimeComposerRegistry(t *testing.T, defs ...middlewarecfg.Definition) middlewarecfg.DefinitionRegistry {
	t.Helper()

	registry := middlewarecfg.NewInMemoryDefinitionRegistry()
	for _, def := range defs {
		if err := registry.RegisterDefinition(def); err != nil {
			t.Fatalf("register middleware definition %q: %v", def.Name(), err)
		}
	}
	return registry
}

func TestRuntimeFingerprint_DoesNotIncludeAPIKeys(t *testing.T) {
	ss, err := settings.NewStepSettings()
	if err != nil {
		t.Fatalf("NewStepSettings: %v", err)
	}
	ss.API.APIKeys["openai"] = "sk-this-should-not-appear"

	fp := buildRuntimeFingerprint("default", 0, "hi", nil, nil, ss)
	if strings.Contains(fp, "sk-this-should-not-appear") {
		t.Fatalf("fingerprint leaked api key: %q", fp)
	}
	if strings.Contains(fp, "\"api_keys\"") || strings.Contains(fp, "\"APIKeys\"") {
		t.Fatalf("fingerprint unexpectedly contains api key fields: %q", fp)
	}
}

func TestWebChatRuntimeComposer_RejectsInvalidOverrideTypes(t *testing.T) {
	composer := newProfileRuntimeComposer(values.New(), newRuntimeComposerRegistry(t), middlewarecfg.BuildDeps{})

	_, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:           "c1",
		ProfileKey:       "default",
		RuntimeOverrides: map[string]any{"middlewares": "bad"},
	})
	if err == nil {
		t.Fatalf("expected error for invalid middlewares override type")
	}
}

func TestWebChatRuntimeComposer_UsesResolvedRuntimeSpec(t *testing.T) {
	composer := newProfileRuntimeComposer(
		minimalRuntimeComposerValues(t),
		newRuntimeComposerRegistry(t),
		middlewarecfg.BuildDeps{},
	)

	res, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:     "c1",
		ProfileKey: "analyst",
		ResolvedProfileRuntime: &gepprofiles.RuntimeSpec{
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
	composer := newProfileRuntimeComposer(
		minimalRuntimeComposerValues(t),
		newRuntimeComposerRegistry(t),
		middlewarecfg.BuildDeps{},
	)

	res, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:     "c1",
		ProfileKey: "analyst",
		ResolvedProfileRuntime: &gepprofiles.RuntimeSpec{
			SystemPrompt: "You are analyst",
		},
		RuntimeOverrides: map[string]any{
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

func TestWebChatRuntimeComposer_UsesResolverPrecedenceForMiddlewareConfig(t *testing.T) {
	var builtConfig map[string]any
	def := &runtimeComposerDefinition{
		name: "agentmode",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"threshold": map[string]any{"type": "integer"},
				"mode":      map[string]any{"type": "string"},
			},
		},
		buildFn: func(_ context.Context, _ middlewarecfg.BuildDeps, cfg any) (gepmiddleware.Middleware, error) {
			builtConfig, _ = cfg.(map[string]any)
			return func(next gepmiddleware.HandlerFunc) gepmiddleware.HandlerFunc { return next }, nil
		},
	}
	composer := newProfileRuntimeComposer(
		minimalRuntimeComposerValues(t),
		newRuntimeComposerRegistry(t, def),
		middlewarecfg.BuildDeps{},
	)

	_, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:     "c1",
		ProfileKey: "analyst",
		ResolvedProfileRuntime: &gepprofiles.RuntimeSpec{
			Middlewares: []gepprofiles.MiddlewareUse{
				{
					Name: "agentmode",
					ID:   "primary",
					Config: map[string]any{
						"threshold": 2,
						"mode":      "safe",
					},
				},
			},
		},
		RuntimeOverrides: map[string]any{
			"middlewares": []any{
				map[string]any{
					"name":   "agentmode",
					"id":     "primary",
					"config": map[string]any{"threshold": "7"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}

	if builtConfig == nil {
		t.Fatalf("expected middleware config to be built")
	}
	if got, want := builtConfig["threshold"], int64(7); got != want {
		t.Fatalf("request override did not win: got=%#v want=%#v", got, want)
	}
	if got, want := builtConfig["mode"], "safe"; got != want {
		t.Fatalf("profile value should be retained when not overridden: got=%#v want=%#v", got, want)
	}
}

func TestWebChatRuntimeComposer_RejectsInvalidMiddlewareSchemaPayload(t *testing.T) {
	def := &runtimeComposerDefinition{
		name: "agentmode",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"threshold": map[string]any{"type": "integer"},
			},
		},
	}
	composer := newProfileRuntimeComposer(
		minimalRuntimeComposerValues(t),
		newRuntimeComposerRegistry(t, def),
		middlewarecfg.BuildDeps{},
	)

	_, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:     "c1",
		ProfileKey: "analyst",
		ResolvedProfileRuntime: &gepprofiles.RuntimeSpec{
			Middlewares: []gepprofiles.MiddlewareUse{
				{
					Name: "agentmode",
					Config: map[string]any{
						"threshold": "not-a-number",
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected schema validation error")
	}
	if !strings.Contains(err.Error(), "resolve middleware agentmode[0]") {
		t.Fatalf("expected middleware instance context in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "/threshold") {
		t.Fatalf("expected path context in schema error, got: %v", err)
	}
}

func TestRuntimeFingerprint_ChangesOnProfileVersion(t *testing.T) {
	fpV1 := buildRuntimeFingerprint("default", 1, "prompt", nil, nil, nil)
	fpV2 := buildRuntimeFingerprint("default", 2, "prompt", nil, nil, nil)
	if fpV1 == fpV2 {
		t.Fatalf("expected fingerprint to change across profile versions")
	}
}
