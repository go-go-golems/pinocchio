package cmds

import (
	"path/filepath"
	"testing"

	"github.com/go-go-golems/glazed/pkg/cmds/values"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

func TestLoadPinocchioProfileRegistryStackRejectsProfileWithoutRegistries(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
	t.Setenv("HOME", tmpDir)

	profileSection, err := profilebootstrap.NewProfileSettingsSection()
	if err != nil {
		t.Fatalf("NewProfileSettingsSection failed: %v", err)
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		t.Fatalf("NewSectionValues failed: %v", err)
	}
	if err := values.WithFieldValue("profile", "analyst")(profileValues); err != nil {
		t.Fatalf("WithFieldValue(profile) failed: %v", err)
	}

	parsed := values.New()
	parsed.Set(profilebootstrap.ProfileSettingsSectionSlug, profileValues)

	_, _, _, err = loadPinocchioProfileRegistryStack(parsed)
	if err == nil {
		t.Fatal("expected profile selection without registries to fail")
	}
	if got := err.Error(); got == "" || got == "analyst" {
		t.Fatalf("expected validation error, got %q", got)
	}
}
