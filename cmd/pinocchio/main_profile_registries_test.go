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
    inference_settings:
      chat:
        api_type: openai
        engine: analyst-model
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
	if !strings.Contains(output, registryPath) {
		t.Fatalf("expected print-parsed-fields output to include profile registry path %q\noutput:\n%s", registryPath, output)
	}
}

func TestProfileFileFlagRemoved(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"go", "run", "./cmd/pinocchio",
		"generate-prompt",
		"--goal", "profile-file removal check",
		"--profile-file", "/tmp/legacy.yaml",
		"--print-parsed-fields",
	)
	cmd.Dir = repoRoot

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected go run pinocchio to fail with unknown --profile-file flag")
	}
	if !strings.Contains(combined.String(), "unknown flag: --profile-file") {
		t.Fatalf("expected unknown --profile-file flag error\noutput:\n%s", combined.String())
	}
}

func TestPinocchioJSInheritsProfileRegistryConfigAndProfileSelection(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "registry.yaml")
	configPath := filepath.Join(tmpDir, "config.yaml")

	registryYAML := `slug: team
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
		t.Fatalf("write registry yaml: %v", err)
	}
	configYAML := "profile-settings:\n  profile-registries: " + registryPath + "\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config yaml: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join("..", ".."))

	t.Run("explicit profile flag after positional script", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx,
			"go", "run", "./cmd/pinocchio",
			"js",
			"./examples/js/runner-profile-smoke.js",
			"--profile", "analyst",
			"--config-file", configPath,
		)
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(),
			"XDG_CONFIG_HOME="+filepath.Join(tmpDir, "xdg"),
		)

		var combined bytes.Buffer
		cmd.Stdout = &combined
		cmd.Stderr = &combined
		if err := cmd.Run(); err != nil {
			t.Fatalf("go run pinocchio js failed: %v\noutput:\n%s", err, combined.String())
		}
		if !strings.Contains(combined.String(), "profile=analyst model=analyst-model prompt=hello from pinocchio js") {
			t.Fatalf("expected analyst profile output\noutput:\n%s", combined.String())
		}
	})

	t.Run("config-backed registry default profile", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx,
			"go", "run", "./cmd/pinocchio",
			"js",
			"./examples/js/runner-profile-smoke.js",
			"--config-file", configPath,
		)
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(),
			"XDG_CONFIG_HOME="+filepath.Join(tmpDir, "xdg"),
		)

		var combined bytes.Buffer
		cmd.Stdout = &combined
		cmd.Stderr = &combined
		if err := cmd.Run(); err != nil {
			t.Fatalf("go run pinocchio js failed: %v\noutput:\n%s", err, combined.String())
		}
		if !strings.Contains(combined.String(), "profile=default model=default-model prompt=hello from pinocchio js") {
			t.Fatalf("expected default profile output\noutput:\n%s", combined.String())
		}
	})
}

func TestPinocchioJSUsesDefaultProfilesYAMLWhenPresent(t *testing.T) {
	tmpDir := t.TempDir()
	xdgHome := filepath.Join(tmpDir, "xdg")
	profilesDir := filepath.Join(xdgHome, "pinocchio")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("mkdir profiles dir: %v", err)
	}
	registryPath := filepath.Join(profilesDir, "profiles.yaml")
	registryYAML := `slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai
        engine: default-model
  gpt-5-mini:
    slug: gpt-5-mini
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5-mini
`
	if err := os.WriteFile(registryPath, []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry yaml: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"go", "run", "./cmd/pinocchio",
		"js",
		"./examples/js/runner-profile-smoke.js",
		"--profile", "gpt-5-mini",
	)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+xdgHome,
		"HOME="+tmpDir,
	)

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run pinocchio js failed: %v\noutput:\n%s", err, combined.String())
	}
	if !strings.Contains(combined.String(), "profile=gpt-5-mini model=gpt-5-mini prompt=hello from pinocchio js") {
		t.Fatalf("expected gpt-5-mini profile output\noutput:\n%s", combined.String())
	}
}
