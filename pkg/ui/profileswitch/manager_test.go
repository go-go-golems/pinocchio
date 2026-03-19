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
