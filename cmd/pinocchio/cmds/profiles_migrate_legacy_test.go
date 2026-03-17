package cmds

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrateLegacyProfilesFile_RejectsLegacyStepSettingsProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "profiles.yaml")
	require.NoError(t, os.WriteFile(inputPath, []byte(`default:
  ai-chat:
    ai-engine: gpt-4o-mini
agent:
  ai-chat:
    ai-engine: gpt-4.1
`), 0o644))

	outPath := filepath.Join(tmpDir, "profiles.runtime.yaml")
	result, err := MigrateLegacyProfilesFile(LegacyProfilesMigrationOptions{
		InputPath:       inputPath,
		OutputPath:      outPath,
		RegistrySlugRaw: "default",
	})
	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "runtime.step_settings_patch has been removed")
}

func TestMigrateLegacyProfilesFile_SkipIfNotLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "profiles.yaml")
	require.NoError(t, os.WriteFile(inputPath, []byte(`registries:
  default:
    slug: default
    default_profile_slug: default
    profiles:
      default:
        slug: default
`), 0o644))

	result, err := MigrateLegacyProfilesFile(LegacyProfilesMigrationOptions{
		InputPath:       inputPath,
		RegistrySlugRaw: "default",
		SkipIfNotLegacy: true,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "canonical-registries", result.InputFormat)
	require.False(t, result.WroteFile)
	require.Empty(t, strings.TrimSpace(string(result.OutputYAML)))
}

func TestMigrateLegacyProfilesFile_RejectsCanonicalBundleWithoutSkip(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "profiles.yaml")
	require.NoError(t, os.WriteFile(inputPath, []byte(`registries:
  default:
    slug: default
    default_profile_slug: default
    profiles:
      default:
        slug: default
`), 0o644))

	result, err := MigrateLegacyProfilesFile(LegacyProfilesMigrationOptions{
		InputPath:       inputPath,
		RegistrySlugRaw: "default",
	})
	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "runtime bundle format")
}

func TestMigrateLegacyProfilesFile_SkipIfNotLegacy_InvalidInputErrors(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "profiles.yaml")
	require.NoError(t, os.WriteFile(inputPath, []byte("registries: ["), 0o644))

	result, err := MigrateLegacyProfilesFile(LegacyProfilesMigrationOptions{
		InputPath:       inputPath,
		RegistrySlugRaw: "default",
		SkipIfNotLegacy: true,
	})
	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported profiles YAML format")
}

func TestMigrateLegacyProfilesFile_SkipIfNotLegacy_EmptyInputErrors(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "profiles.yaml")
	require.NoError(t, os.WriteFile(inputPath, []byte("   \n"), 0o644))

	result, err := MigrateLegacyProfilesFile(LegacyProfilesMigrationOptions{
		InputPath:       inputPath,
		RegistrySlugRaw: "default",
		SkipIfNotLegacy: true,
	})
	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is empty")
}
