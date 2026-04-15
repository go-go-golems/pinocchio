package configdoc

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeDocument_ValidMinimalDocument(t *testing.T) {
	doc, err := DecodeDocument([]byte(`
app:
  repositories:
    - ~/prompts/base
profile:
  active: Assistant
  registries:
    - ' ~/.pinocchio/profiles.yaml '
profiles:
  Default:
    display_name: Default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini
`))
	if err != nil {
		t.Fatalf("DecodeDocument failed: %v", err)
	}
	if len(doc.App.Repositories) != 1 || doc.App.Repositories[0] != "~/prompts/base" {
		t.Fatalf("unexpected repositories: %#v", doc.App.Repositories)
	}
	if got := doc.Profile.Active; got != "assistant" {
		t.Fatalf("expected normalized active profile, got %q", got)
	}
	if len(doc.Profile.Registries) != 1 || doc.Profile.Registries[0] != "~/.pinocchio/profiles.yaml" {
		t.Fatalf("unexpected registries: %#v", doc.Profile.Registries)
	}
	if _, ok := doc.Profiles["default"]; !ok {
		t.Fatalf("expected normalized profile slug map, got %#v", doc.Profiles)
	}
}

func TestDecodeDocument_RejectsLegacyTopLevelRuntimeSection(t *testing.T) {
	_, err := DecodeDocument([]byte(`
ai-chat:
  ai-engine: gpt-5-mini
`))
	if err == nil {
		t.Fatal("expected old top-level runtime section to fail")
	}
	if !strings.Contains(err.Error(), "field ai-chat not found") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestDecodeDocument_RejectsLegacyProfileSettings(t *testing.T) {
	_, err := DecodeDocument([]byte(`
profile-settings:
  profile: analyst
`))
	if err == nil {
		t.Fatal("expected legacy profile-settings to fail")
	}
	if !strings.Contains(err.Error(), "field profile-settings not found") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestDecodeDocument_RejectsDuplicateNormalizedProfileSlugs(t *testing.T) {
	_, err := DecodeDocument([]byte(`
profiles:
  Default:
    display_name: Default
  default:
    display_name: Duplicate
`))
	if err == nil {
		t.Fatal("expected duplicate normalized slug to fail")
	}
	if !strings.Contains(err.Error(), "duplicate profile slug") {
		t.Fatalf("expected duplicate slug error, got %v", err)
	}
}

func TestDecodeDocument_RejectsEmptyRegistryEntry(t *testing.T) {
	_, err := DecodeDocument([]byte(`
profile:
  registries:
    - ""
`))
	if err == nil {
		t.Fatal("expected empty registry entry to fail")
	}
	if !strings.Contains(err.Error(), "profile.registries[0] cannot be empty") {
		t.Fatalf("expected empty-registry error, got %v", err)
	}
}

func TestLoadDocument_RejectsLegacyLocalOverrideFilename(t *testing.T) {
	tmpDir := t.TempDir()
	legacyPath := filepath.Join(tmpDir, LegacyLocalOverrideFileName)
	_, err := LoadDocument(legacyPath)
	if err == nil {
		t.Fatal("expected legacy local filename to fail")
	}
	if !strings.Contains(err.Error(), LegacyLocalOverrideFileName) {
		t.Fatalf("expected legacy filename error, got %v", err)
	}
}
