package configdoc

import (
	"os"
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
	if !strings.Contains(err.Error(), "unsupported legacy top-level keys") {
		t.Fatalf("expected top-level-key guidance, got %v", err)
	}
	if !strings.Contains(err.Error(), "ai-chat") {
		t.Fatalf("expected legacy key name in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "profiles.<slug>.inference_settings") {
		t.Fatalf("expected migration hint in error, got %v", err)
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
	if !strings.Contains(err.Error(), "profile-settings") {
		t.Fatalf("expected legacy key in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "profile.active") {
		t.Fatalf("expected migration target in error, got %v", err)
	}
}

func TestLoadDocument_RejectsLegacyKeysWithFilePathAndStrictDecodeExplanation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`profile-settings:
  profile: analyst
ai-chat:
  ai-engine: gpt-5-mini
repositories:
  - ~/prompts
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err = LoadDocument(configPath)
	if err == nil {
		t.Fatal("expected LoadDocument to fail on legacy keys")
	}
	for _, needle := range []string{
		configPath,
		"profile-settings",
		"ai-chat",
		"repositories",
		"app.repositories",
		"supported top-level keys are: app, profile, profiles",
		"not treated as optional or ignored",
	} {
		if !strings.Contains(err.Error(), needle) {
			t.Fatalf("expected %q in error, got %v", needle, err)
		}
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
