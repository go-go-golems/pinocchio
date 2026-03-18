package profileswitch

import (
	"context"
	"testing"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

func TestManagerResolveDefaultProfile(t *testing.T) {
	ctx := context.Background()

	store := gepprofiles.NewInMemoryProfileStore()
	reg := &gepprofiles.ProfileRegistry{
		Slug:               gepprofiles.MustRegistrySlug("default"),
		DefaultProfileSlug: gepprofiles.MustProfileSlug("default"),
		Profiles: map[gepprofiles.ProfileSlug]*gepprofiles.Profile{
			gepprofiles.MustProfileSlug("default"): {
				Slug: gepprofiles.MustProfileSlug("default"),
				Runtime: gepprofiles.RuntimeSpec{
					SystemPrompt: "hello",
				},
				Metadata: gepprofiles.ProfileMetadata{Version: 3},
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
	if res.ProfileVersion != 3 {
		t.Fatalf("ProfileVersion=%d, want 3", res.ProfileVersion)
	}
}
