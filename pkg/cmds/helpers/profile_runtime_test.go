package helpers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

func TestResolveFinalInferenceSettings_MergesBaseConfigWithSelectedEngineProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
	t.Setenv("HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "pinocchio-config.yaml")
	configYAML := `
ai-chat:
  ai-api-type: openai
  ai-engine: base-model
ai-client:
  timeout: 42
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	registryPath := filepath.Join(tmpDir, "profiles.yaml")
	registryYAML := `
slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai-responses
        engine: gpt-5-mini
      inference:
        reasoning_effort: medium
`
	if err := os.WriteFile(registryPath, []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	parsed, err := buildTestParsedValues(configPath, "default", registryPath)
	if err != nil {
		t.Fatalf("build parsed values: %v", err)
	}

	resolved, err := ResolveFinalInferenceSettings(context.Background(), parsed)
	if err != nil {
		t.Fatalf("ResolveFinalInferenceSettings failed: %v", err)
	}
	if resolved.Close != nil {
		defer resolved.Close()
	}
	if resolved.InferenceSettings == nil || resolved.InferenceSettings.Chat == nil || resolved.InferenceSettings.Chat.Engine == nil {
		t.Fatal("expected resolved inference settings with chat engine")
	}
	if got := *resolved.InferenceSettings.Chat.Engine; got != "gpt-5-mini" {
		t.Fatalf("expected profile-selected engine, got %q", got)
	}
	if resolved.ResolvedEngineProfile == nil || resolved.ResolvedEngineProfile.EngineProfileSlug.String() != "default" {
		t.Fatalf("expected resolved engine profile metadata, got %#v", resolved.ResolvedEngineProfile)
	}
	if len(resolved.ConfigFiles) == 0 || resolved.ConfigFiles[len(resolved.ConfigFiles)-1] != configPath {
		t.Fatalf("expected explicit config file to be tracked, got %#v", resolved.ConfigFiles)
	}
}

func TestResolveFinalInferenceSettings_UsesBaseConfigWhenNoRegistryConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
	t.Setenv("HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "pinocchio-config.yaml")
	configYAML := `
ai-chat:
  ai-api-type: openai
  ai-engine: base-model
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	parsed, err := buildTestParsedValues(configPath, "", "")
	if err != nil {
		t.Fatalf("build parsed values: %v", err)
	}

	resolved, err := ResolveFinalInferenceSettings(context.Background(), parsed)
	if err != nil {
		t.Fatalf("ResolveFinalInferenceSettings failed: %v", err)
	}
	if resolved.ResolvedEngineProfile != nil {
		t.Fatalf("expected no resolved engine profile, got %#v", resolved.ResolvedEngineProfile)
	}
	if resolved.InferenceSettings == nil || resolved.InferenceSettings.Chat == nil || resolved.InferenceSettings.Chat.Engine == nil {
		t.Fatal("expected base inference settings with chat engine")
	}
	if got := *resolved.InferenceSettings.Chat.Engine; got != "base-model" {
		t.Fatalf("expected base model, got %q", got)
	}
}

func buildTestParsedValues(configPath string, profile string, profileRegistries string) (*values.Values, error) {
	ret := values.New()

	commandSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}
	commandValues, err := values.NewSectionValues(commandSection)
	if err != nil {
		return nil, err
	}
	if configPath != "" {
		if err := values.WithFieldValue("config-file", configPath)(commandValues); err != nil {
			return nil, err
		}
	}
	ret.Set(cli.CommandSettingsSlug, commandValues)

	profileSection, err := NewProfileSettingsSection()
	if err != nil {
		return nil, err
	}
	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		return nil, err
	}
	if profile != "" {
		if err := values.WithFieldValue("profile", profile)(profileValues); err != nil {
			return nil, err
		}
	}
	if profileRegistries != "" {
		if err := values.WithFieldValue("profile-registries", profileRegistries)(profileValues); err != nil {
			return nil, err
		}
	}
	ret.Set(ProfileSettingsSectionSlug, profileValues)

	return ret, nil
}
