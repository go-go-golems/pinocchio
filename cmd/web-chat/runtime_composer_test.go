package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	gepmiddleware "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	aitypes "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

func runtimeSpecWithTestAPIKey(spec *infruntime.ProfileRuntime) *infruntime.ProfileRuntime {
	if spec == nil {
		spec = &infruntime.ProfileRuntime{}
	}
	return &infruntime.ProfileRuntime{
		SystemPrompt: spec.SystemPrompt,
		Middlewares:  append([]infruntime.MiddlewareUse(nil), spec.Middlewares...),
		Tools:        append([]string(nil), spec.Tools...),
	}
}

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

func testBaseInferenceSettings(t *testing.T) *settings.InferenceSettings {
	t.Helper()

	ss, err := settings.NewInferenceSettings()
	if err != nil {
		t.Fatalf("NewInferenceSettings: %v", err)
	}
	ss.Chat.Engine = ptr("base-engine")
	ss.Chat.ApiType = apiTypePtr(aitypes.ApiTypeOpenAI)
	ss.API.APIKeys["openai-api-key"] = "base-api-key"
	ss.API.BaseUrls["openai-base-url"] = "https://api.openai.com/v1"
	return ss
}

func testResolvedInferenceSettings(t *testing.T, mutate func(*settings.InferenceSettings)) *settings.InferenceSettings {
	t.Helper()

	ss := testBaseInferenceSettings(t)
	if mutate != nil {
		mutate(ss)
	}
	return ss
}

func TestRuntimeFingerprint_DoesNotIncludeAPIKeys(t *testing.T) {
	ss := testBaseInferenceSettings(t)
	ss.API.APIKeys["openai"] = "sk-this-should-not-appear"

	fp := buildRuntimeFingerprint("default", 0, "hi", nil, nil, ss)
	if strings.Contains(fp, "sk-this-should-not-appear") {
		t.Fatalf("fingerprint leaked api key: %q", fp)
	}
	if strings.Contains(fp, "\"api_keys\"") || strings.Contains(fp, "\"APIKeys\"") {
		t.Fatalf("fingerprint unexpectedly contains api key fields: %q", fp)
	}
}

func TestWebChatRuntimeComposer_UsesResolvedRuntimeSpec(t *testing.T) {
	composer := newProfileRuntimeComposer(
		newRuntimeComposerRegistry(t),
		middlewarecfg.BuildDeps{},
		nil,
	)

	res, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:                    "c1",
		ProfileKey:                "analyst",
		ResolvedInferenceSettings: testResolvedInferenceSettings(t, nil),
		ResolvedProfileRuntime: runtimeSpecWithTestAPIKey(&infruntime.ProfileRuntime{
			SystemPrompt: "You are analyst",
			Tools:        []string{"calculator", "  "},
		}),
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	if res.SeedSystemPrompt != "You are analyst" {
		t.Fatalf("unexpected seed prompt: %q", res.SeedSystemPrompt)
	}
}

func TestWebChatRuntimeComposer_UsesBaseInferenceSettingsWhenResolvedRuntimeIsEmpty(t *testing.T) {
	composer := newProfileRuntimeComposer(
		newRuntimeComposerRegistry(t),
		middlewarecfg.BuildDeps{},
		testBaseInferenceSettings(t),
	)

	res, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:                    "c1",
		ProfileKey:                "analyst",
		ResolvedInferenceSettings: testResolvedInferenceSettings(t, nil),
		ResolvedProfileRuntime:    &infruntime.ProfileRuntime{},
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(res.RuntimeFingerprint), &payload); err != nil {
		t.Fatalf("unmarshal runtime fingerprint: %v", err)
	}
	stepMeta, ok := payload["step_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("missing step_metadata in runtime fingerprint: %#v", payload)
	}
	if got, want := stepMeta["ai-engine"], "base-engine"; got != want {
		t.Fatalf("base inference settings were not used: got=%#v want=%#v", got, want)
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
		newRuntimeComposerRegistry(t, def),
		middlewarecfg.BuildDeps{},
		nil,
	)

	_, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:                    "c1",
		ProfileKey:                "analyst",
		ResolvedInferenceSettings: testResolvedInferenceSettings(t, nil),
		ResolvedProfileRuntime: runtimeSpecWithTestAPIKey(&infruntime.ProfileRuntime{
			Middlewares: []infruntime.MiddlewareUse{
				{
					Name: "agentmode",
					ID:   "primary",
					Config: map[string]any{
						"threshold": 2,
						"mode":      "safe",
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}

	if builtConfig == nil {
		t.Fatalf("expected middleware config to be built")
	}
	if got, want := builtConfig["threshold"], int64(2); got != want {
		t.Fatalf("profile middleware config mismatch: got=%#v want=%#v", got, want)
	}
	if got, want := builtConfig["mode"], "safe"; got != want {
		t.Fatalf("profile value mismatch: got=%#v want=%#v", got, want)
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
		newRuntimeComposerRegistry(t, def),
		middlewarecfg.BuildDeps{},
		nil,
	)

	_, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:                    "c1",
		ProfileKey:                "analyst",
		ResolvedInferenceSettings: testResolvedInferenceSettings(t, nil),
		ResolvedProfileRuntime: runtimeSpecWithTestAPIKey(&infruntime.ProfileRuntime{
			Middlewares: []infruntime.MiddlewareUse{
				{
					Name: "agentmode",
					Config: map[string]any{
						"threshold": "not-a-number",
					},
				},
			},
		}),
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

func TestWebChatRuntimeComposer_PrefersResolvedProfileFingerprint(t *testing.T) {
	composer := newProfileRuntimeComposer(
		newRuntimeComposerRegistry(t),
		middlewarecfg.BuildDeps{},
		nil,
	)

	res, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:                     "c1",
		ProfileKey:                 "analyst",
		ResolvedInferenceSettings:  testResolvedInferenceSettings(t, nil),
		ResolvedProfileFingerprint: "sha256:resolver-owned",
		ResolvedProfileRuntime: runtimeSpecWithTestAPIKey(&infruntime.ProfileRuntime{
			SystemPrompt: "profile prompt",
		}),
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	if got, want := res.RuntimeFingerprint, "sha256:resolver-owned"; got != want {
		t.Fatalf("runtime fingerprint mismatch: got=%q want=%q", got, want)
	}
}

func TestWebChatRuntimeComposer_UsesResolvedInferenceSettings(t *testing.T) {
	composer := newProfileRuntimeComposer(
		newRuntimeComposerRegistry(t),
		middlewarecfg.BuildDeps{},
		nil,
	)

	res, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:                    "c1",
		ProfileKey:                "analyst",
		ResolvedInferenceSettings: testResolvedInferenceSettings(t, func(ss *settings.InferenceSettings) { ss.Chat.Engine = ptr("patched-engine") }),
		ResolvedProfileRuntime:    runtimeSpecWithTestAPIKey(&infruntime.ProfileRuntime{}),
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(res.RuntimeFingerprint), &payload); err != nil {
		t.Fatalf("unmarshal runtime fingerprint: %v", err)
	}
	stepMeta, ok := payload["step_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("missing step_metadata in runtime fingerprint: %#v", payload)
	}
	if got, want := stepMeta["ai-engine"], "patched-engine"; got != want {
		t.Fatalf("resolved inference settings not used: got=%#v want=%#v", got, want)
	}
}

func TestWebChatRuntimeComposer_ResolvedInferenceSettingsOverrideBase(t *testing.T) {
	composer := newProfileRuntimeComposer(
		newRuntimeComposerRegistry(t),
		middlewarecfg.BuildDeps{},
		testBaseInferenceSettings(t),
	)

	res, err := composer.Compose(context.Background(), infruntime.ConversationRuntimeRequest{
		ConvID:     "c1",
		ProfileKey: "analyst",
		ResolvedInferenceSettings: testResolvedInferenceSettings(t, func(ss *settings.InferenceSettings) {
			ss.Chat.Engine = ptr("gpt-5-nano")
			ss.Chat.ApiType = apiTypePtr(aitypes.ApiTypeOpenAIResponses)
		}),
		ResolvedProfileRuntime: &infruntime.ProfileRuntime{},
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(res.RuntimeFingerprint), &payload); err != nil {
		t.Fatalf("unmarshal runtime fingerprint: %v", err)
	}
	stepMeta, ok := payload["step_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("missing step_metadata in runtime fingerprint: %#v", payload)
	}
	if got, want := stepMeta["ai-engine"], "gpt-5-nano"; got != want {
		t.Fatalf("resolved inference settings did not drive ai-engine: got=%#v want=%#v", got, want)
	}
	if got, want := stepMeta["ai-api-type"], "openai-responses"; got != want {
		t.Fatalf("resolved inference settings did not drive ai-api-type: got=%#v want=%#v", got, want)
	}
}

func ptr[T any](v T) *T {
	return &v
}

func apiTypePtr(v aitypes.ApiType) *aitypes.ApiType {
	return &v
}
