package cmds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	geppettobootstrap "github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
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

	profileSettings, _, err := profilebootstrap.ResolveEngineProfileSettings(parsed)
	if err != nil {
		t.Fatalf("ResolveEngineProfileSettings failed: %v", err)
	}
	_, err = geppettobootstrap.ResolveProfileRegistryChain(context.Background(), profileSettings)
	if err == nil {
		t.Fatal("expected profile selection without registries to fail")
	}
	if got := err.Error(); got == "" || got == "analyst" {
		t.Fatalf("expected validation error, got %q", got)
	}
}

func TestNewJSCommand_ExposesProfileFlags(t *testing.T) {
	cmd := NewJSCommand()
	if cmd.Flags().Lookup("profile") == nil {
		t.Fatal("expected --profile flag on js command")
	}
	if cmd.Flags().Lookup("profile-registries") == nil {
		t.Fatal("expected --profile-registries flag on js command")
	}
	if cmd.Flags().Lookup("print-parsed-fields") == nil {
		t.Fatal("expected --print-parsed-fields flag on js command")
	}
	if cmd.Flags().Lookup("print-inference-settings") == nil {
		t.Fatal("expected --print-inference-settings flag on js command")
	}
}

func TestResolvePinocchioJSRuntimeBootstrap_UsesFinalInferenceSettingsFromSelectedProfile(t *testing.T) {
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
`
	if err := os.WriteFile(registryPath, []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	parsed, err := profilebootstrap.NewCLISelectionValues(profilebootstrap.CLISelectionInput{
		ConfigFile:        configPath,
		Profile:           "default",
		ProfileRegistries: []string{registryPath},
	})
	if err != nil {
		t.Fatalf("NewCLISelectionValues failed: %v", err)
	}

	resolved, err := resolvePinocchioJSRuntimeBootstrap(context.Background(), parsed)
	if err != nil {
		t.Fatalf("resolvePinocchioJSRuntimeBootstrap failed: %v", err)
	}
	if resolved.Close != nil {
		defer resolved.Close()
	}

	if resolved.DefaultInferenceSettings == nil || resolved.DefaultInferenceSettings.Chat == nil || resolved.DefaultInferenceSettings.Chat.Engine == nil {
		t.Fatal("expected resolved default inference settings with chat engine")
	}
	if got := *resolved.DefaultInferenceSettings.Chat.Engine; got != "gpt-5-mini" {
		t.Fatalf("expected selected profile engine in JS defaults, got %q", got)
	}
	if !resolved.UseDefaultProfileResolve {
		t.Fatal("expected JS runtime bootstrap to enable default profile resolution")
	}
}
