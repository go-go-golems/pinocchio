package configdoc

import (
	"os"
	"path/filepath"
	"testing"

	glazedconfig "github.com/go-go-golems/glazed/pkg/config"
)

func writeResolvedDocFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestLoadResolvedDocuments_MergesUserRepoCWDAndExplicitInOrder(t *testing.T) {
	tmpDir := t.TempDir()
	userPath := writeResolvedDocFixture(t, tmpDir, "user.yaml", `
app:
  repositories:
    - ~/prompts/base
profile:
  active: default
  registries:
    - ~/.pinocchio/user.yaml
profiles:
  default:
    display_name: User Default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5
`)
	repoPath := writeResolvedDocFixture(t, tmpDir, "repo.yaml", `
app:
  repositories:
    - ./repo-prompts
profiles:
  default:
    inference_settings:
      chat:
        engine: gpt-5-mini
`)
	cwdPath := writeResolvedDocFixture(t, tmpDir, "cwd.yaml", `
app:
  repositories:
    - ./cwd-prompts
profile:
  active: assistant
profiles:
  assistant:
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: llama-local
`)
	explicitPath := writeResolvedDocFixture(t, tmpDir, "explicit.yaml", `
profile:
  registries:
    - ~/.pinocchio/explicit.yaml
`)

	resolved, err := LoadResolvedDocuments([]glazedconfig.ResolvedConfigFile{
		{Path: userPath, Layer: glazedconfig.LayerUser, SourceName: "user", Index: 0},
		{Path: repoPath, Layer: glazedconfig.LayerRepo, SourceName: "repo", Index: 1},
		{Path: cwdPath, Layer: glazedconfig.LayerCWD, SourceName: "cwd", Index: 2},
		{Path: explicitPath, Layer: glazedconfig.LayerExplicit, SourceName: "explicit", Index: 3},
	})
	if err != nil {
		t.Fatalf("LoadResolvedDocuments failed: %v", err)
	}
	if len(resolved.Documents) != 4 {
		t.Fatalf("expected 4 decoded documents, got %d", len(resolved.Documents))
	}
	effective := resolved.Effective
	if effective == nil {
		t.Fatal("expected effective merged document")
	}
	wantRepos := []string{"~/prompts/base", "./repo-prompts", "./cwd-prompts"}
	if len(effective.App.Repositories) != len(wantRepos) {
		t.Fatalf("unexpected repositories: got=%#v want=%#v", effective.App.Repositories, wantRepos)
	}
	for i := range wantRepos {
		if effective.App.Repositories[i] != wantRepos[i] {
			t.Fatalf("repository[%d] mismatch: got=%q want=%q", i, effective.App.Repositories[i], wantRepos[i])
		}
	}
	if got := effective.Profile.Active; got != "assistant" {
		t.Fatalf("expected cwd profile.active override, got %q", got)
	}
	if len(effective.Profile.Registries) != 1 || effective.Profile.Registries[0] != "~/.pinocchio/explicit.yaml" {
		t.Fatalf("expected explicit registries replacement, got %#v", effective.Profile.Registries)
	}
	defaultProfile := effective.Profiles["default"]
	if defaultProfile == nil || defaultProfile.InferenceSettings == nil || defaultProfile.InferenceSettings.Chat == nil || defaultProfile.InferenceSettings.Chat.Engine == nil {
		t.Fatalf("expected merged default profile, got %#v", defaultProfile)
	}
	if got := *defaultProfile.InferenceSettings.Chat.Engine; got != "gpt-5-mini" {
		t.Fatalf("expected repo override for default profile engine, got %q", got)
	}
	assistantProfile := effective.Profiles["assistant"]
	if assistantProfile == nil || assistantProfile.InferenceSettings == nil || assistantProfile.InferenceSettings.Chat == nil || assistantProfile.InferenceSettings.Chat.Engine == nil {
		t.Fatalf("expected assistant profile, got %#v", assistantProfile)
	}
	if got := *assistantProfile.InferenceSettings.Chat.Engine; got != "llama-local" {
		t.Fatalf("expected cwd assistant profile engine, got %q", got)
	}
}
