package cmds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/credentials"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	aitypes "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
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

	_, err = profilebootstrap.ResolveCLIProfileRuntime(context.Background(), parsed)
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
	if cmd.Flags().Lookup("turns-dsn") == nil {
		t.Fatal("expected --turns-dsn flag on js command")
	}
	if cmd.Flags().Lookup("turns-db") == nil {
		t.Fatal("expected --turns-db flag on js command")
	}
}

type jsHostOnlyBearerSource struct{}

func (jsHostOnlyBearerSource) BearerToken(context.Context, credentials.Request) (string, error) {
	// Construction must not resolve a bearer. Returning no credential material
	// keeps this wiring test independent of secret-shaped fixture values.
	return "", nil
}

func TestPinocchioJSRuntimeForwardsHostBearerSourceToBothEngineBuilders(t *testing.T) {
	defaults, err := aisettings.NewInferenceSettings()
	if err != nil {
		t.Fatalf("NewInferenceSettings: %v", err)
	}
	apiType := aitypes.ApiTypeOpenAI
	model := "test-model"
	defaults.Chat.ApiType = &apiType
	defaults.Chat.Engine = &model
	defaults.API.APIKeys = map[string]string{}

	profilePath := filepath.Join(t.TempDir(), "profiles.yaml")
	if err := os.WriteFile(profilePath, []byte(`slug: workspace
profiles:
  assistant:
    inference_settings:
      api:
        api_keys: {}
      chat:
        api_type: openai
        engine: test-model
`), 0o600); err != nil {
		t.Fatalf("write profiles: %v", err)
	}

	rt, err := newPinocchioJSRuntime(context.Background(), pinocchioJSRuntimeOptions{
		ScriptDir:                t.TempDir(),
		DefaultInferenceSettings: defaults,
		BearerTokenSource:        jsHostOnlyBearerSource{},
	})
	if err != nil {
		t.Fatalf("newPinocchioJSRuntime: %v", err)
	}
	defer func() { _ = rt.Close(context.Background()) }()
	if err := rt.VM.Set("profilePath", profilePath); err != nil {
		t.Fatalf("set profilePath: %v", err)
	}

	_, err = rt.VM.RunString(`
		const gp = require("geppetto");
		const pp = require("pinocchio");
		const registry = gp.inferenceProfiles.load(globalThis.profilePath);
		const settings = registry.resolve("assistant");
		gp.engine().inference(settings).build();
		pp.engines.fromDefaults();
		if (Object.prototype.hasOwnProperty.call(gp, "bearerTokenSource")) throw new Error("geppetto source exposed");
		if (Object.prototype.hasOwnProperty.call(pp, "bearerTokenSource")) throw new Error("pinocchio source exposed");
	`)
	if err != nil {
		t.Fatalf("source-aware JS engine construction failed: %v", err)
	}
}

func TestResolvePinocchioJSRuntimeBootstrap_UsesFinalInferenceSettingsFromSelectedProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
	t.Setenv("HOME", tmpDir)

	configPath := filepath.Join(tmpDir, "pinocchio-config.yaml")
	configYAML := `
profile:
  active: default
profiles:
  default:
    inference_settings:
      chat:
        api_type: openai-responses
        engine: gpt-5-mini
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	parsed, err := profilebootstrap.NewCLISelectionValues(profilebootstrap.CLISelectionInput{
		ConfigFile: configPath,
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
