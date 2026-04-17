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

func TestResolveRepositoryPaths_MergesUserRepoAndCWDUnifiedConfigRepositories(t *testing.T) {
	repoDir, cwdDir, restore := setupRepositoryGitWorkspace(t)
	defer restore()

	tmpHome := t.TempDir()
	homeDir := filepath.Join(tmpHome, "home")
	xdgDir := filepath.Join(tmpHome, "xdg")
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	homeConfig := filepath.Join(homeDir, ".pinocchio", "config.yaml")
	xdgConfig := filepath.Join(xdgDir, "pinocchio", "config.yaml")
	repoConfig := filepath.Join(repoDir, ".pinocchio.yml")
	repoOverrideConfig := filepath.Join(repoDir, ".pinocchio.override.yml")
	cwdConfig := filepath.Join(cwdDir, ".pinocchio.yml")
	cwdOverrideConfig := filepath.Join(cwdDir, ".pinocchio.override.yml")
	for path, body := range map[string]string{
		homeConfig:         "app:\n  repositories:\n    - /home/base\n    - /shared\n",
		xdgConfig:          "app:\n  repositories:\n    - /xdg/extra\n",
		repoConfig:         "app:\n  repositories:\n    - /repo/prompts\n    - /shared\n",
		repoOverrideConfig: "app:\n  repositories:\n    - /repo/private\n",
		cwdConfig:          "app:\n  repositories:\n    - /cwd/prompts\n",
		cwdOverrideConfig:  "app:\n  repositories:\n    - /cwd/private\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	repositories, err := ResolveRepositoryPaths()
	if err != nil {
		t.Fatalf("ResolveRepositoryPaths failed: %v", err)
	}
	want := []string{"/home/base", "/shared", "/xdg/extra", "/repo/prompts", "/repo/private", "/cwd/prompts", "/cwd/private"}
	if len(repositories) != len(want) {
		t.Fatalf("unexpected repositories length: got=%#v want=%#v", repositories, want)
	}
	for i := range want {
		if repositories[i] != want[i] {
			t.Fatalf("repository[%d] mismatch: got=%q want=%q", i, repositories[i], want[i])
		}
	}
}

func setupRepositoryGitWorkspace(t *testing.T) (string, string, func()) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	restore := func() { _ = os.Chdir(oldWD) }

	repoDir := t.TempDir()
	if err := runRepositoryGit(repoDir, "init"); err != nil {
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

func runRepositoryGit(dir string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(scrubGitEnv(os.Environ()), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return nil
}
