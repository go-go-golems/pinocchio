package cmds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/glazed/pkg/cmds/values"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

func TestLoadPinocchioProfileRegistryStackUsesImplicitDefaultRegistryFallback(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	t.Setenv("HOME", tmpDir)

	registryPath := filepath.Join(xdgDir, "pinocchio", "profiles.yaml")
	if err := os.MkdirAll(filepath.Dir(registryPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	registryYAML := `
slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai
        engine: default-model
  analyst:
    slug: analyst
    inference_settings:
      chat:
        api_type: openai
        engine: analyst-model
`
	if err := os.WriteFile(registryPath, []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

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

	reader, defaultResolve, closer, err := loadPinocchioProfileRegistryStack(parsed)
	if err != nil {
		t.Fatalf("loadPinocchioProfileRegistryStack failed: %v", err)
	}
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}
	if reader == nil {
		t.Fatal("expected registry reader")
	}
	resolved, err := reader.ResolveEngineProfile(context.Background(), defaultResolve)
	if err != nil {
		t.Fatalf("ResolveEngineProfile failed: %v", err)
	}
	if got := resolved.EngineProfileSlug.String(); got != "analyst" {
		t.Fatalf("expected analyst profile, got %q", got)
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
