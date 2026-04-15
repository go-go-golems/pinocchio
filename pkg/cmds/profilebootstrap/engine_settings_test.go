package profilebootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCLIEngineSettingsFromBase_MatchesResolvedPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
	t.Setenv("HOME", tmpDir)
	t.Setenv("PINOCCHIO_AI_ENGINE", "env-model")

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

	parsed, err := NewCLISelectionValues(CLISelectionInput{
		ConfigFile: configPath,
	})
	if err != nil {
		t.Fatalf("NewCLISelectionValues failed: %v", err)
	}

	base, configFiles, err := ResolveBaseInferenceSettings(parsed)
	if err != nil {
		t.Fatalf("ResolveBaseInferenceSettings failed: %v", err)
	}

	resolvedDirect, err := ResolveCLIEngineSettings(context.Background(), parsed)
	if err != nil {
		t.Fatalf("ResolveCLIEngineSettings failed: %v", err)
	}
	if resolvedDirect.Close != nil {
		defer resolvedDirect.Close()
	}

	resolvedFromBase, err := ResolveCLIEngineSettingsFromBase(context.Background(), base, parsed, configFiles)
	if err != nil {
		t.Fatalf("ResolveCLIEngineSettingsFromBase failed: %v", err)
	}
	if resolvedFromBase.Close != nil {
		defer resolvedFromBase.Close()
	}

	if resolvedDirect.BaseInferenceSettings == nil || resolvedDirect.BaseInferenceSettings.Chat == nil || resolvedDirect.BaseInferenceSettings.Chat.Engine == nil {
		t.Fatal("expected direct resolution base engine")
	}
	if resolvedFromBase.BaseInferenceSettings == nil || resolvedFromBase.BaseInferenceSettings.Chat == nil || resolvedFromBase.BaseInferenceSettings.Chat.Engine == nil {
		t.Fatal("expected from-base resolution base engine")
	}
	if got, want := *resolvedFromBase.BaseInferenceSettings.Chat.Engine, *resolvedDirect.BaseInferenceSettings.Chat.Engine; got != want {
		t.Fatalf("base engine mismatch: got %q want %q", got, want)
	}
	if got := *resolvedDirect.BaseInferenceSettings.Chat.Engine; got != "env-model" {
		t.Fatalf("expected env-model base engine, got %q", got)
	}

	if resolvedDirect.FinalInferenceSettings == nil || resolvedDirect.FinalInferenceSettings.Chat == nil || resolvedDirect.FinalInferenceSettings.Chat.Engine == nil || resolvedDirect.FinalInferenceSettings.Chat.ApiType == nil {
		t.Fatal("expected direct resolution final chat settings")
	}
	if resolvedFromBase.FinalInferenceSettings == nil || resolvedFromBase.FinalInferenceSettings.Chat == nil || resolvedFromBase.FinalInferenceSettings.Chat.Engine == nil || resolvedFromBase.FinalInferenceSettings.Chat.ApiType == nil {
		t.Fatal("expected from-base resolution final chat settings")
	}
	if got, want := *resolvedFromBase.FinalInferenceSettings.Chat.Engine, *resolvedDirect.FinalInferenceSettings.Chat.Engine; got != want {
		t.Fatalf("final engine mismatch: got %q want %q", got, want)
	}
	if got, want := string(*resolvedFromBase.FinalInferenceSettings.Chat.ApiType), string(*resolvedDirect.FinalInferenceSettings.Chat.ApiType); got != want {
		t.Fatalf("final api type mismatch: got %q want %q", got, want)
	}
	if got, want := resolvedFromBase.ProfileSelection.Profile, resolvedDirect.ProfileSelection.Profile; got != want {
		t.Fatalf("profile mismatch: got %q want %q", got, want)
	}
}
