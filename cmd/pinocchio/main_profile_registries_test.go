package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrintParsedFields_ProfileRegistriesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "registry.yaml")
	configPath := filepath.Join(tmpDir, "config.yaml")

	registryYAML := `slug: team
profiles:
  analyst:
    slug: analyst
    runtime:
      step_settings_patch:
        ai-chat:
          ai-engine: team-analyst
`
	if err := os.WriteFile(registryPath, []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry yaml: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write config yaml: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"go", "run", "./cmd/pinocchio",
		"generate-prompt",
		"--log-level", "error",
		"--goal", "profile registries parsed fields smoke",
		"--profile-registries", registryPath,
		"--profile", "analyst",
		"--config-file", configPath,
		"--print-parsed-fields",
	)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(tmpDir, "xdg"),
	)

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run pinocchio failed: %v\noutput:\n%s", err, combined.String())
	}

	output := combined.String()
	if !strings.Contains(output, "profile-settings:") {
		t.Fatalf("expected print-parsed-fields output to include profile-settings section\noutput:\n%s", output)
	}
	if !strings.Contains(output, "mode: profile-registry-stack") {
		t.Fatalf("expected print-parsed-fields output to include profile-registry-stack marker\noutput:\n%s", output)
	}
	if !strings.Contains(output, "profileRegistries:") {
		t.Fatalf("expected print-parsed-fields output to include profileRegistries metadata\noutput:\n%s", output)
	}
	if !strings.Contains(output, registryPath) {
		t.Fatalf("expected print-parsed-fields output to include profile registry path %q\noutput:\n%s", registryPath, output)
	}
	if !strings.Contains(output, "value: team-analyst") {
		t.Fatalf("expected ai-engine value from profile registry\noutput:\n%s", output)
	}
}
