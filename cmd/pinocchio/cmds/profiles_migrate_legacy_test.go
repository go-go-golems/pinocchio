package cmds

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrateLegacyProfilesFile_WritesCanonicalOutput(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "profiles.yaml")
	require.NoError(t, os.WriteFile(inputPath, []byte(`default:
  ai-chat:
    ai-engine: gpt-4o-mini
agent:
  ai-chat:
    ai-engine: gpt-4.1
`), 0o644))

	outPath := filepath.Join(tmpDir, "profiles.registry.yaml")
	result, err := MigrateLegacyProfilesFile(LegacyProfilesMigrationOptions{
		InputPath:       inputPath,
		OutputPath:      outPath,
		RegistrySlugRaw: "default",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.WroteFile)
	require.Equal(t, "legacy-map", result.InputFormat)
	require.Equal(t, 1, result.RegistryCount)
	require.Equal(t, 2, result.ProfileCount)

	out, err := os.ReadFile(outPath)
	require.NoError(t, err)
	outS := string(out)
	require.Contains(t, outS, "registries:")
	require.Contains(t, outS, "default_profile_slug: default")
	require.Contains(t, outS, "step_settings_patch:")
	require.Contains(t, outS, "agent:")
}

func TestMigrateLegacyProfilesFile_InPlaceCreatesBackup(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "profiles.yaml")
	original := `default:
  ai-chat:
    ai-engine: gpt-4o-mini
`
	require.NoError(t, os.WriteFile(inputPath, []byte(original), 0o644))

	result, err := MigrateLegacyProfilesFile(LegacyProfilesMigrationOptions{
		InputPath:       inputPath,
		RegistrySlugRaw: "default",
		InPlace:         true,
		BackupInPlace:   true,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.WroteFile)
	require.NotEmpty(t, result.CreatedBackupPath)

	backup, err := os.ReadFile(result.CreatedBackupPath)
	require.NoError(t, err)
	require.Equal(t, original, string(backup))

	migrated, err := os.ReadFile(inputPath)
	require.NoError(t, err)
	require.Contains(t, string(migrated), "registries:")
	require.NotEqual(t, original, string(migrated))
}

func TestMigrateLegacyProfilesFile_DryRunDoesNotWriteOutput(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "profiles.yaml")
	require.NoError(t, os.WriteFile(inputPath, []byte(`default:
  ai-chat:
    ai-engine: gpt-4o-mini
`), 0o644))

	outPath := filepath.Join(tmpDir, "profiles.registry.yaml")
	result, err := MigrateLegacyProfilesFile(LegacyProfilesMigrationOptions{
		InputPath:       inputPath,
		OutputPath:      outPath,
		RegistrySlugRaw: "default",
		DryRun:          true,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.WroteFile)
	require.Contains(t, string(result.OutputYAML), "registries:")
	_, statErr := os.Stat(outPath)
	require.Error(t, statErr)
	require.True(t, os.IsNotExist(statErr))
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
