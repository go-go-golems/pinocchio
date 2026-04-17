package profilebootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCLIConfigFilesResolved_UsesRepoCWDAndExplicitOrder(t *testing.T) {
	repoDir, cwdDir, restore := setupGitWorkspace(t)
	defer restore()

	tmpHome := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmpHome, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, "xdg"))

	repoFile := filepath.Join(repoDir, ".pinocchio.yml")
	repoOverrideFile := filepath.Join(repoDir, ".pinocchio.override.yml")
	cwdFile := filepath.Join(cwdDir, ".pinocchio.yml")
	cwdOverrideFile := filepath.Join(cwdDir, ".pinocchio.override.yml")
	explicitFile := filepath.Join(repoDir, "explicit.yaml")
	for _, entry := range []struct {
		path    string
		content string
	}{
		{repoFile, "profile:\n  active: repo-profile\n"},
		{repoOverrideFile, "profile:\n  active: repo-override-profile\n"},
		{cwdFile, "profile:\n  active: cwd-profile\n"},
		{cwdOverrideFile, "profile:\n  active: cwd-override-profile\n"},
		{explicitFile, "profile:\n  active: explicit-profile\n"},
	} {
		if err := os.WriteFile(entry.path, []byte(entry.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", entry.path, err)
		}
	}

	parsed, err := NewCLISelectionValues(CLISelectionInput{ConfigFile: explicitFile})
	if err != nil {
		t.Fatalf("NewCLISelectionValues failed: %v", err)
	}

	resolved, err := ResolveCLIConfigFilesResolved(parsed)
	if err != nil {
		t.Fatalf("ResolveCLIConfigFilesResolved failed: %v", err)
	}
	want := []string{repoFile, repoOverrideFile, cwdFile, cwdOverrideFile, explicitFile}
	if len(resolved.Files) != len(want) {
		t.Fatalf("config file count mismatch: got=%#v want=%#v", resolved.Files, want)
	}
	for i := range want {
		if resolved.Files[i].Path != want[i] {
			t.Fatalf("config file[%d] mismatch: got=%q want=%q", i, resolved.Files[i].Path, want[i])
		}
	}
}

func TestResolveCLIProfileSelection_CWDOverridesRepoAndExplicitWins(t *testing.T) {
	repoDir, cwdDir, restore := setupGitWorkspace(t)
	defer restore()

	tmpHome := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmpHome, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, "xdg"))

	repoFile := filepath.Join(repoDir, ".pinocchio.yml")
	repoOverrideFile := filepath.Join(repoDir, ".pinocchio.override.yml")
	cwdFile := filepath.Join(cwdDir, ".pinocchio.yml")
	cwdOverrideFile := filepath.Join(cwdDir, ".pinocchio.override.yml")
	explicitFile := filepath.Join(repoDir, "explicit.yaml")
	for _, entry := range []struct {
		path    string
		content string
	}{
		{repoFile, "profile:\n  active: repo-profile\n"},
		{repoOverrideFile, "profile:\n  active: repo-override-profile\n"},
		{cwdFile, "profile:\n  active: cwd-profile\n"},
		{cwdOverrideFile, "profile:\n  active: cwd-override-profile\n"},
		{explicitFile, "profile:\n  active: explicit-profile\n"},
	} {
		if err := os.WriteFile(entry.path, []byte(entry.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", entry.path, err)
		}
	}

	resolved, err := ResolveCLIProfileSelection(nil)
	if err != nil {
		t.Fatalf("ResolveCLIProfileSelection(nil) failed: %v", err)
	}
	if got := resolved.Profile; got != "cwd-override-profile" {
		t.Fatalf("expected cwd override profile to override repo and cwd base profiles, got %q", got)
	}

	parsed, err := NewCLISelectionValues(CLISelectionInput{ConfigFile: explicitFile})
	if err != nil {
		t.Fatalf("NewCLISelectionValues failed: %v", err)
	}
	resolved, err = ResolveCLIProfileSelection(parsed)
	if err != nil {
		t.Fatalf("ResolveCLIProfileSelection(parsed) failed: %v", err)
	}
	if got := resolved.Profile; got != "explicit-profile" {
		t.Fatalf("expected explicit profile to override cwd/repo profiles, got %q", got)
	}
}

func TestResolveBaseInferenceSettings_IgnoresUnifiedConfigRuntimeFieldsAndKeepsConfigFiles(t *testing.T) {
	repoDir, cwdDir, restore := setupGitWorkspace(t)
	defer restore()

	tmpHome := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmpHome, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, "xdg"))
	t.Setenv("PINOCCHIO_AI_ENGINE", "env-model")

	repoFile := filepath.Join(repoDir, ".pinocchio.yml")
	cwdFile := filepath.Join(cwdDir, ".pinocchio.yml")
	explicitFile := filepath.Join(repoDir, "explicit.yaml")
	for _, entry := range []struct {
		path    string
		content string
	}{
		{repoFile, "profile:\n  active: repo-profile\nprofiles:\n  repo-profile:\n    inference_settings:\n      chat:\n        api_type: openai\n        engine: repo-model\n"},
		{cwdFile, "profile:\n  active: cwd-profile\nprofiles:\n  cwd-profile:\n    inference_settings:\n      chat:\n        engine: cwd-model\n"},
		{explicitFile, "profile:\n  active: explicit-profile\nprofiles:\n  explicit-profile:\n    inference_settings:\n      chat:\n        engine: explicit-model\n"},
	} {
		if err := os.WriteFile(entry.path, []byte(entry.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", entry.path, err)
		}
	}

	parsed, err := NewCLISelectionValues(CLISelectionInput{ConfigFile: explicitFile})
	if err != nil {
		t.Fatalf("NewCLISelectionValues failed: %v", err)
	}

	settings, files, err := ResolveBaseInferenceSettings(parsed)
	if err != nil {
		t.Fatalf("ResolveBaseInferenceSettings failed: %v", err)
	}
	if settings.Chat == nil || settings.Chat.Engine == nil {
		t.Fatal("expected resolved chat engine")
	}
	if got := *settings.Chat.Engine; got != "env-model" {
		t.Fatalf("expected env engine to remain base, got %q", got)
	}
	wantFiles := []string{repoFile, cwdFile, explicitFile}
	if len(files) != len(wantFiles) {
		t.Fatalf("config files mismatch: got=%#v want=%#v", files, wantFiles)
	}
	for i := range wantFiles {
		if files[i] != wantFiles[i] {
			t.Fatalf("config file[%d] mismatch: got=%q want=%q", i, files[i], wantFiles[i])
		}
	}
}

func TestResolveUnifiedConfig_ExposesExplainData(t *testing.T) {
	repoDir, cwdDir, restore := setupGitWorkspace(t)
	defer restore()

	tmpHome := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmpHome, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, "xdg"))

	repoFile := filepath.Join(repoDir, ".pinocchio.yml")
	cwdFile := filepath.Join(cwdDir, ".pinocchio.yml")
	for _, entry := range []struct {
		path    string
		content string
	}{
		{repoFile, "profile:\n  active: repo-profile\napp:\n  repositories:\n    - ./repo-prompts\n"},
		{cwdFile, "profile:\n  active: cwd-profile\napp:\n  repositories:\n    - ./cwd-prompts\n"},
	} {
		if err := os.WriteFile(entry.path, []byte(entry.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", entry.path, err)
		}
	}

	resolved, err := ResolveUnifiedConfig(nil)
	if err != nil {
		t.Fatalf("ResolveUnifiedConfig failed: %v", err)
	}
	if resolved.Documents == nil || resolved.Documents.Explain == nil {
		t.Fatal("expected resolved explain data")
	}
	activeEntries := resolved.Documents.Explain.Entries("profile.active")
	if len(activeEntries) != 2 {
		t.Fatalf("expected two profile.active explain entries, got %#v", activeEntries)
	}
	if activeEntries[1].File.Path != cwdFile {
		t.Fatalf("expected cwd active override provenance, got %#v", activeEntries[1])
	}
}

func TestResolveCLIEngineSettings_InlineProfileKeepsBaseValuesWhenOmittingFields(t *testing.T) {
	_, cwdDir, restore := setupGitWorkspace(t)
	defer restore()

	tmpHome := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmpHome, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, "xdg"))
	t.Setenv("PINOCCHIO_AI_ENGINE", "env-model")

	cwdFile := filepath.Join(cwdDir, ".pinocchio.yml")
	if err := os.WriteFile(cwdFile, []byte("profile:\n  active: analyst\nprofiles:\n  analyst:\n    inference_settings:\n      api_keys:\n        api_keys:\n          openai-api-key: inline-key\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", cwdFile, err)
	}

	resolved, err := ResolveCLIEngineSettings(context.Background(), nil)
	if err != nil {
		t.Fatalf("ResolveCLIEngineSettings failed: %v", err)
	}
	if resolved.Close != nil {
		defer resolved.Close()
	}
	if resolved.FinalInferenceSettings == nil || resolved.FinalInferenceSettings.Chat == nil || resolved.FinalInferenceSettings.Chat.Engine == nil {
		t.Fatal("expected final inference settings")
	}
	if got := *resolved.FinalInferenceSettings.Chat.Engine; got != "env-model" {
		t.Fatalf("expected omitted inline engine to stay at base env-model, got %q", got)
	}
	if got := resolved.FinalInferenceSettings.API.APIKeys["openai-api-key"]; got != "inline-key" {
		t.Fatalf("expected inline api key override, got %q", got)
	}
}

func TestResolveCLIEngineSettings_UsesMergedDocumentSelectionAndInlineProfiles(t *testing.T) {
	repoDir, cwdDir, restore := setupGitWorkspace(t)
	defer restore()

	tmpHome := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmpHome, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, "xdg"))
	t.Setenv("PINOCCHIO_AI_ENGINE", "env-model")

	repoFile := filepath.Join(repoDir, ".pinocchio.yml")
	cwdFile := filepath.Join(cwdDir, ".pinocchio.yml")
	for _, entry := range []struct {
		path    string
		content string
	}{
		{repoFile, "profile:\n  active: assistant\nprofiles:\n  default:\n    inference_settings:\n      chat:\n        api_type: openai-responses\n        engine: gpt-5\n"},
		{cwdFile, "profiles:\n  assistant:\n    stack:\n      - profile_slug: default\n    inference_settings:\n      chat:\n        engine: gpt-5-mini\n"},
	} {
		if err := os.WriteFile(entry.path, []byte(entry.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", entry.path, err)
		}
	}

	resolved, err := ResolveCLIEngineSettings(context.Background(), nil)
	if err != nil {
		t.Fatalf("ResolveCLIEngineSettings failed: %v", err)
	}
	if resolved.Close != nil {
		defer resolved.Close()
	}
	if resolved.ProfileSelection == nil || resolved.ProfileSelection.Profile != "assistant" {
		t.Fatalf("expected assistant profile selection, got %#v", resolved.ProfileSelection)
	}
	if resolved.BaseInferenceSettings == nil || resolved.BaseInferenceSettings.Chat == nil || resolved.BaseInferenceSettings.Chat.Engine == nil {
		t.Fatal("expected base inference settings")
	}
	if got := *resolved.BaseInferenceSettings.Chat.Engine; got != "env-model" {
		t.Fatalf("expected env-model base engine, got %q", got)
	}
	if resolved.FinalInferenceSettings == nil || resolved.FinalInferenceSettings.Chat == nil || resolved.FinalInferenceSettings.Chat.Engine == nil {
		t.Fatal("expected final inference settings")
	}
	if got := *resolved.FinalInferenceSettings.Chat.Engine; got != "gpt-5-mini" {
		t.Fatalf("expected inline profile engine, got %q", got)
	}
	if resolved.FinalInferenceSettings.Chat.ApiType == nil || string(*resolved.FinalInferenceSettings.Chat.ApiType) != "openai-responses" {
		t.Fatalf("expected stacked api_type to be preserved, got %#v", resolved.FinalInferenceSettings.Chat.ApiType)
	}
}

func setupGitWorkspace(t *testing.T) (string, string, func()) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	restore := func() { _ = os.Chdir(oldWD) }

	repoDir := t.TempDir()
	if err := runGit(repoDir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	cwdDir := filepath.Join(repoDir, "sub", "dir")
	if err := os.MkdirAll(cwdDir, 0o755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatalf("chdir cwd: %v", err)
	}
	return repoDir, cwdDir, restore
}

func runGit(dir string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return nil
}
