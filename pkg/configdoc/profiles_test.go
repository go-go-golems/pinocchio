package configdoc

import (
	"context"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
)

func TestInlineProfilesToRegistry_BuildsRegistryAndChoosesDefault(t *testing.T) {
	doc := mustDecodeDocument(t, `
profiles:
  assistant:
    display_name: Assistant
    inference_settings:
      chat:
        engine: gpt-5-mini
  default:
    display_name: Default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5
`)

	reg, err := InlineProfilesToRegistry(doc, "")
	if err != nil {
		t.Fatalf("InlineProfilesToRegistry failed: %v", err)
	}
	if got := reg.Slug.String(); got != DefaultInlineRegistrySlug {
		t.Fatalf("expected default inline registry slug, got %q", got)
	}
	if got := reg.DefaultEngineProfileSlug.String(); got != "default" {
		t.Fatalf("expected default profile slug, got %q", got)
	}
	if len(reg.Profiles) != 2 {
		t.Fatalf("expected 2 inline profiles, got %#v", reg.Profiles)
	}
	assistant := reg.Profiles[gepprofiles.MustEngineProfileSlug("assistant")]
	if assistant == nil || assistant.DisplayName != "Assistant" {
		t.Fatalf("unexpected assistant profile: %#v", assistant)
	}
}

func TestNewInlineStoreRegistry_ResolvesStackedInlineProfiles(t *testing.T) {
	doc := mustDecodeDocument(t, `
profiles:
  default:
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5
  assistant:
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5-mini
`)

	registry, err := NewInlineStoreRegistry(doc, gepprofiles.MustRegistrySlug("inline-workspace"))
	if err != nil {
		t.Fatalf("NewInlineStoreRegistry failed: %v", err)
	}

	resolved, err := registry.ResolveEngineProfile(context.Background(), gepprofiles.ResolveInput{
		EngineProfileSlug: gepprofiles.MustEngineProfileSlug("assistant"),
	})
	if err != nil {
		t.Fatalf("ResolveEngineProfile failed: %v", err)
	}
	if got := resolved.RegistrySlug.String(); got != "inline-workspace" {
		t.Fatalf("expected inline registry slug, got %q", got)
	}
	if resolved.InferenceSettings == nil || resolved.InferenceSettings.Chat == nil || resolved.InferenceSettings.Chat.Engine == nil {
		t.Fatal("expected resolved inference settings with chat engine")
	}
	if got := *resolved.InferenceSettings.Chat.Engine; got != "gpt-5-mini" {
		t.Fatalf("expected stacked inline engine override, got %q", got)
	}
	if resolved.InferenceSettings.Chat.ApiType == nil || *resolved.InferenceSettings.Chat.ApiType != "openai" {
		t.Fatalf("expected stacked inline api_type to be preserved, got %#v", resolved.InferenceSettings.Chat.ApiType)
	}
}

func TestComposeRegistry_InlineWinsSameSlugAndImportedRemainsAvailable(t *testing.T) {
	inlineDoc := mustDecodeDocument(t, `
profiles:
  assistant:
    inference_settings:
      chat:
        engine: gpt-5-mini
`)
	inlineRegistry, err := NewInlineStoreRegistry(inlineDoc, gepprofiles.MustRegistrySlug("config-inline"))
	if err != nil {
		t.Fatalf("NewInlineStoreRegistry failed: %v", err)
	}

	importedDoc := mustDecodeDocument(t, `
profiles:
  assistant:
    inference_settings:
      chat:
        engine: imported-model
  analyst:
    inference_settings:
      chat:
        engine: analyst-model
`)
	importedRegistry, err := NewInlineStoreRegistry(importedDoc, gepprofiles.MustRegistrySlug("team-profiles"))
	if err != nil {
		t.Fatalf("NewInlineStoreRegistry(imported) failed: %v", err)
	}

	composed := ComposeRegistry(importedRegistry, inlineRegistry)
	if composed == nil {
		t.Fatal("expected composed registry")
	}

	resolved, err := composed.ResolveEngineProfile(context.Background(), gepprofiles.ResolveInput{
		EngineProfileSlug: gepprofiles.MustEngineProfileSlug("assistant"),
	})
	if err != nil {
		t.Fatalf("ResolveEngineProfile(assistant) failed: %v", err)
	}
	if resolved.InferenceSettings == nil || resolved.InferenceSettings.Chat == nil || resolved.InferenceSettings.Chat.Engine == nil {
		t.Fatal("expected assistant inference settings")
	}
	if got := *resolved.InferenceSettings.Chat.Engine; got != "gpt-5-mini" {
		t.Fatalf("expected inline same-slug profile to win, got %q", got)
	}
	if got := resolved.RegistrySlug.String(); got != "config-inline" {
		t.Fatalf("expected inline registry slug, got %q", got)
	}

	resolved, err = composed.ResolveEngineProfile(context.Background(), gepprofiles.ResolveInput{
		EngineProfileSlug: gepprofiles.MustEngineProfileSlug("analyst"),
	})
	if err != nil {
		t.Fatalf("ResolveEngineProfile(analyst) failed: %v", err)
	}
	if resolved.InferenceSettings == nil || resolved.InferenceSettings.Chat == nil || resolved.InferenceSettings.Chat.Engine == nil {
		t.Fatal("expected analyst inference settings")
	}
	if got := *resolved.InferenceSettings.Chat.Engine; got != "analyst-model" {
		t.Fatalf("expected imported profile fallback, got %q", got)
	}
	if got := resolved.RegistrySlug.String(); got != "team-profiles" {
		t.Fatalf("expected imported registry slug, got %q", got)
	}
}

func TestComposeRegistry_EmptyResolveInputFallsBackToImportedDefault(t *testing.T) {
	inlineDoc := mustDecodeDocument(t, `
profiles:
  assistant:
    inference_settings:
      chat:
        engine: gpt-5-mini
`)
	inlineRegistry, err := NewInlineStoreRegistry(inlineDoc, gepprofiles.MustRegistrySlug("config-inline"))
	if err != nil {
		t.Fatalf("NewInlineStoreRegistry(inline) failed: %v", err)
	}

	importedDoc := mustDecodeDocument(t, `
profiles:
  default:
    inference_settings:
      chat:
        engine: imported-default
`)
	importedRegistry, err := NewInlineStoreRegistry(importedDoc, gepprofiles.MustRegistrySlug("team-profiles"))
	if err != nil {
		t.Fatalf("NewInlineStoreRegistry(imported) failed: %v", err)
	}

	composed := ComposeRegistry(importedRegistry, inlineRegistry)
	resolved, err := composed.ResolveEngineProfile(context.Background(), gepprofiles.ResolveInput{})
	if err != nil {
		t.Fatalf("ResolveEngineProfile(empty) failed: %v", err)
	}
	if got := resolved.RegistrySlug.String(); got != "team-profiles" {
		t.Fatalf("expected imported default registry to handle empty input, got %q", got)
	}
	if got := resolved.EngineProfileSlug.String(); got != "default" {
		t.Fatalf("expected imported default profile to handle empty input, got %q", got)
	}
}
