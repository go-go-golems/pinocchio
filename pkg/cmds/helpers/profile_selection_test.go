package helpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

func TestResolveCLIProfileSelection_MergesConfigAndExplicitValues(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
	t.Setenv("HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "pinocchio-config.yaml")
	configYAML := `
profile-settings:
  profile: config-default
  profile-registries:
    - ` + filepath.Join(tmpDir, "config-registry.yaml") + `
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	parsed, err := buildTestParsedValues(configPath, "cli-profile", filepath.Join(tmpDir, "cli-registry.yaml"))
	if err != nil {
		t.Fatalf("build parsed values: %v", err)
	}

	resolved, err := ResolveCLIProfileSelection(parsed)
	if err != nil {
		t.Fatalf("ResolveCLIProfileSelection failed: %v", err)
	}

	if got := resolved.Profile; got != "cli-profile" {
		t.Fatalf("expected explicit profile to win, got %q", got)
	}
	if len(resolved.ProfileRegistries) != 1 || resolved.ProfileRegistries[0] != filepath.Join(tmpDir, "cli-registry.yaml") {
		t.Fatalf("expected explicit registries to win, got %#v", resolved.ProfileRegistries)
	}
	if len(resolved.ConfigFiles) != 1 || resolved.ConfigFiles[0] != configPath {
		t.Fatalf("expected config file to be tracked, got %#v", resolved.ConfigFiles)
	}
}

func TestResolveCLIProfileSelection_UsesConfigWhenExplicitValuesMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
	t.Setenv("HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "pinocchio-config.yaml")
	registryPath := filepath.Join(tmpDir, "config-registry.yaml")
	configYAML := `
profile-settings:
  profile: config-default
  profile-registries:
    - ` + registryPath + `
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	parsed, err := buildTestParsedValues(configPath, "", "")
	if err != nil {
		t.Fatalf("build parsed values: %v", err)
	}

	resolved, err := ResolveCLIProfileSelection(parsed)
	if err != nil {
		t.Fatalf("ResolveCLIProfileSelection failed: %v", err)
	}

	if got := resolved.Profile; got != "config-default" {
		t.Fatalf("expected config profile, got %q", got)
	}
	if len(resolved.ProfileRegistries) != 1 || resolved.ProfileRegistries[0] != registryPath {
		t.Fatalf("expected config registries, got %#v", resolved.ProfileRegistries)
	}
}

func TestResolveCLIProfileSelection_UsesDefaultRegistryFallbackWhenUnset(t *testing.T) {
	tmpDir := t.TempDir()
	xdgConfig := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("HOME", tmpDir)

	registryPath := filepath.Join(xdgConfig, "pinocchio", "profiles.yaml")
	if err := os.MkdirAll(filepath.Dir(registryPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(registryPath, []byte("slug: workspace\nprofiles: {}\n"), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	commandSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		t.Fatalf("command section: %v", err)
	}
	commandValues, err := values.NewSectionValues(commandSection)
	if err != nil {
		t.Fatalf("command values: %v", err)
	}

	parsed := values.New()
	parsed.Set(cli.CommandSettingsSlug, commandValues)

	resolved, err := ResolveCLIProfileSelection(parsed)
	if err != nil {
		t.Fatalf("ResolveCLIProfileSelection failed: %v", err)
	}

	if len(resolved.ProfileRegistries) != 1 || resolved.ProfileRegistries[0] != registryPath {
		t.Fatalf("expected XDG registry fallback, got %#v", resolved.ProfileRegistries)
	}
}
