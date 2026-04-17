package profileswitch

import (
	"context"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	aitypes "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
)

func TestManagerResolveDefaultProfile(t *testing.T) {
	ctx := context.Background()

	store := gepprofiles.NewInMemoryEngineProfileStore()
	reg := &gepprofiles.EngineProfileRegistry{
		Slug:                     gepprofiles.MustRegistrySlug("default"),
		DefaultEngineProfileSlug: gepprofiles.MustEngineProfileSlug("default"),
		Profiles: map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{
			gepprofiles.MustEngineProfileSlug("default"): {
				Slug:              gepprofiles.MustEngineProfileSlug("default"),
				InferenceSettings: mustTestInferenceSettings(t, aitypes.ApiTypeOpenAIResponses, "gpt-5-mini"),
				Metadata:          gepprofiles.EngineProfileMetadata{Version: 3},
			},
		},
	}
	if err := store.UpsertRegistry(ctx, reg, gepprofiles.SaveOptions{Actor: "test", Source: "test"}); err != nil {
		t.Fatalf("UpsertRegistry failed: %v", err)
	}
	service, err := gepprofiles.NewStoreRegistry(store, gepprofiles.MustRegistrySlug("default"))
	if err != nil {
		t.Fatalf("NewStoreRegistry failed: %v", err)
	}

	base, err := settings.NewInferenceSettings()
	if err != nil {
		t.Fatalf("NewInferenceSettings failed: %v", err)
	}

	mgr, err := NewManager(service, "", base)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	res, err := mgr.Resolve(ctx, "")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if res.ProfileSlug.String() != "default" {
		t.Fatalf("ProfileSlug=%q, want %q", res.ProfileSlug.String(), "default")
	}
	if res.RegistrySlug.String() != "default" {
		t.Fatalf("RegistrySlug=%q, want %q", res.RegistrySlug.String(), "default")
	}
	if res.InferenceSettings == nil {
		t.Fatalf("InferenceSettings is nil")
	}
	if res.InferenceSettings.Chat == nil || res.InferenceSettings.Chat.Engine == nil || *res.InferenceSettings.Chat.Engine != "gpt-5-mini" {
		t.Fatalf("expected merged model gpt-5-mini, got %#v", res.InferenceSettings.Chat)
	}
	if res.ProfileVersion != 3 {
		t.Fatalf("ProfileVersion=%d, want 3", res.ProfileVersion)
	}
}

func TestManagerSwitch_RebuildsFromBaseInsteadOfKeepingPriorProfileOverrides(t *testing.T) {
	ctx := context.Background()

	store := gepprofiles.NewInMemoryEngineProfileStore()
	reg := &gepprofiles.EngineProfileRegistry{
		Slug:                     gepprofiles.MustRegistrySlug("default"),
		DefaultEngineProfileSlug: gepprofiles.MustEngineProfileSlug("alpha"),
		Profiles: map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{
			gepprofiles.MustEngineProfileSlug("alpha"): {
				Slug:              gepprofiles.MustEngineProfileSlug("alpha"),
				InferenceSettings: mustTestInferenceSettings(t, aitypes.ApiTypeOpenAIResponses, "alpha-model"),
			},
			gepprofiles.MustEngineProfileSlug("beta"): {
				Slug: gepprofiles.MustEngineProfileSlug("beta"),
				InferenceSettings: &settings.InferenceSettings{
					API: &settings.APISettings{
						APIKeys:  map[string]string{"openai-api-key": "beta-key"},
						BaseUrls: map[string]string{},
					},
				},
			},
		},
	}
	if err := store.UpsertRegistry(ctx, reg, gepprofiles.SaveOptions{Actor: "test", Source: "test"}); err != nil {
		t.Fatalf("UpsertRegistry failed: %v", err)
	}
	service, err := gepprofiles.NewStoreRegistry(store, gepprofiles.MustRegistrySlug("default"))
	if err != nil {
		t.Fatalf("NewStoreRegistry failed: %v", err)
	}

	base, err := settings.NewInferenceSettings()
	if err != nil {
		t.Fatalf("NewInferenceSettings failed: %v", err)
	}
	baseApiType := aitypes.ApiTypeOpenAI
	base.Chat.ApiType = &baseApiType
	baseEngine := "base-model"
	base.Chat.Engine = &baseEngine
	base.API.APIKeys["openai-api-key"] = "base-key"

	mgr, err := NewManager(service, "", base)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	alpha, err := mgr.Switch(ctx, "alpha")
	if err != nil {
		t.Fatalf("Switch(alpha) failed: %v", err)
	}
	if alpha.InferenceSettings == nil || alpha.InferenceSettings.Chat == nil || alpha.InferenceSettings.Chat.Engine == nil {
		t.Fatal("expected alpha inference settings")
	}
	if got := *alpha.InferenceSettings.Chat.Engine; got != "alpha-model" {
		t.Fatalf("expected alpha engine override, got %q", got)
	}

	beta, err := mgr.Switch(ctx, "beta")
	if err != nil {
		t.Fatalf("Switch(beta) failed: %v", err)
	}
	if beta.InferenceSettings == nil || beta.InferenceSettings.Chat == nil || beta.InferenceSettings.Chat.Engine == nil {
		t.Fatal("expected beta inference settings")
	}
	if got := *beta.InferenceSettings.Chat.Engine; got != "base-model" {
		t.Fatalf("expected switch to rebuild from base engine, got %q", got)
	}
	if beta.InferenceSettings.Chat.ApiType == nil || *beta.InferenceSettings.Chat.ApiType != aitypes.ApiTypeOpenAI {
		t.Fatalf("expected base api type to be preserved, got %#v", beta.InferenceSettings.Chat.ApiType)
	}
	if got := beta.InferenceSettings.API.APIKeys["openai-api-key"]; got != "beta-key" {
		t.Fatalf("expected beta api key override, got %q", got)
	}
}

func TestManagerResolve_LeavesBaseValuesWhenProfileOmitsThem(t *testing.T) {
	ctx := context.Background()

	store := gepprofiles.NewInMemoryEngineProfileStore()
	reg := &gepprofiles.EngineProfileRegistry{
		Slug:                     gepprofiles.MustRegistrySlug("default"),
		DefaultEngineProfileSlug: gepprofiles.MustEngineProfileSlug("default"),
		Profiles: map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{
			gepprofiles.MustEngineProfileSlug("default"): {
				Slug: gepprofiles.MustEngineProfileSlug("default"),
				InferenceSettings: &settings.InferenceSettings{
					API: &settings.APISettings{
						APIKeys:  map[string]string{"openai-api-key": "profile-key"},
						BaseUrls: map[string]string{},
					},
				},
			},
		},
	}
	if err := store.UpsertRegistry(ctx, reg, gepprofiles.SaveOptions{Actor: "test", Source: "test"}); err != nil {
		t.Fatalf("UpsertRegistry failed: %v", err)
	}
	service, err := gepprofiles.NewStoreRegistry(store, gepprofiles.MustRegistrySlug("default"))
	if err != nil {
		t.Fatalf("NewStoreRegistry failed: %v", err)
	}

	base, err := settings.NewInferenceSettings()
	if err != nil {
		t.Fatalf("NewInferenceSettings failed: %v", err)
	}
	baseApiType := aitypes.ApiTypeOpenAI
	base.Chat.ApiType = &baseApiType
	baseEngine := "base-model"
	base.Chat.Engine = &baseEngine
	base.API.APIKeys["openai-api-key"] = "base-key"

	mgr, err := NewManager(service, "", base)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	res, err := mgr.Resolve(ctx, "default")
	if err != nil {
		t.Fatalf("Resolve(default) failed: %v", err)
	}
	if res.InferenceSettings == nil || res.InferenceSettings.Chat == nil || res.InferenceSettings.Chat.Engine == nil {
		t.Fatal("expected resolved inference settings")
	}
	if got := *res.InferenceSettings.Chat.Engine; got != "base-model" {
		t.Fatalf("expected omitted engine to stay at base value, got %q", got)
	}
	if got := res.InferenceSettings.API.APIKeys["openai-api-key"]; got != "profile-key" {
		t.Fatalf("expected profile api key override, got %q", got)
	}
}

func mustTestInferenceSettings(t *testing.T, apiType aitypes.ApiType, model string) *settings.InferenceSettings {
	t.Helper()
	ss, err := settings.NewInferenceSettings()
	if err != nil {
		t.Fatalf("NewInferenceSettings failed: %v", err)
	}
	ss.Chat.ApiType = &apiType
	ss.Chat.Engine = &model
	return ss
}
