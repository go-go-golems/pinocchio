package profilebootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRepositoryPathsUsesHighestPrecedenceConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))

	homeConfig := filepath.Join(tmpDir, ".pinocchio", "config.yaml")
	xdgConfig := filepath.Join(tmpDir, "xdg", "pinocchio", "config.yaml")
	for path, body := range map[string]string{
		homeConfig: "repositories:\n  - /home/only\n",
		xdgConfig:  "repositories:\n  - /xdg/first\n  - /xdg/second\n",
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
	if len(repositories) != 2 || repositories[0] != "/xdg/first" || repositories[1] != "/xdg/second" {
		t.Fatalf("unexpected repositories: %#v", repositories)
	}
}
