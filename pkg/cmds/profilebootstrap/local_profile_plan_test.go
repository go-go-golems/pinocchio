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

	repoFile := filepath.Join(repoDir, ".pinocchio-profile.yml")
	cwdFile := filepath.Join(cwdDir, ".pinocchio-profile.yml")
	explicitFile := filepath.Join(repoDir, "explicit.yaml")
	for _, entry := range []struct {
		path    string
		content string
	}{
		{repoFile, "profile-settings:\n  profile: repo-profile\n"},
		{cwdFile, "profile-settings:\n  profile: cwd-profile\n"},
		{explicitFile, "profile-settings:\n  profile: explicit-profile\n"},
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
	want := []string{repoFile, cwdFile, explicitFile}
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

	repoFile := filepath.Join(repoDir, ".pinocchio-profile.yml")
	cwdFile := filepath.Join(cwdDir, ".pinocchio-profile.yml")
	explicitFile := filepath.Join(repoDir, "explicit.yaml")
	for _, entry := range []struct {
		path    string
		content string
	}{
		{repoFile, "profile-settings:\n  profile: repo-profile\n"},
		{cwdFile, "profile-settings:\n  profile: cwd-profile\n"},
		{explicitFile, "profile-settings:\n  profile: explicit-profile\n"},
	} {
		if err := os.WriteFile(entry.path, []byte(entry.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", entry.path, err)
		}
	}

	resolved, err := ResolveCLIProfileSelection(nil)
	if err != nil {
		t.Fatalf("ResolveCLIProfileSelection(nil) failed: %v", err)
	}
	if got := resolved.Profile; got != "cwd-profile" {
		t.Fatalf("expected cwd profile to override repo profile, got %q", got)
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

func TestResolveBaseInferenceSettings_UsesRepoCWDAndExplicitConfigPrecedence(t *testing.T) {
	repoDir, cwdDir, restore := setupGitWorkspace(t)
	defer restore()

	tmpHome := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmpHome, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, "xdg"))

	repoFile := filepath.Join(repoDir, ".pinocchio-profile.yml")
	cwdFile := filepath.Join(cwdDir, ".pinocchio-profile.yml")
	explicitFile := filepath.Join(repoDir, "explicit.yaml")
	for _, entry := range []struct {
		path    string
		content string
	}{
		{repoFile, "ai-chat:\n  ai-api-type: openai\n  ai-engine: repo-model\n"},
		{cwdFile, "ai-chat:\n  ai-engine: cwd-model\n"},
		{explicitFile, "ai-chat:\n  ai-engine: explicit-model\n"},
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
	if got := *settings.Chat.Engine; got != "explicit-model" {
		t.Fatalf("expected explicit engine to win, got %q", got)
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

func setupGitWorkspace(t *testing.T) (repoDir string, cwdDir string, restore func()) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	restore = func() { _ = os.Chdir(oldWD) }

	repoDir = t.TempDir()
	if err := runGit(repoDir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	cwdDir = filepath.Join(repoDir, "sub", "dir")
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
