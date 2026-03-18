package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	cmdhelpers "github.com/go-go-golems/pinocchio/pkg/cmds/helpers"
)

func main() {
	var (
		inputPath     string
		outputPath    string
		registrySlug  string
		inPlace       bool
		force         bool
		backupInPlace bool
		dryRun        bool
	)

	flag.StringVar(&inputPath, "input", "", "Input profiles YAML path (defaults to ~/.config/pinocchio/profiles.yaml)")
	flag.StringVar(&outputPath, "output", "", "Output path (defaults to <input>.engine-profiles.yaml unless --in-place)")
	flag.StringVar(&registrySlug, "registry", "", "Fallback registry slug for legacy profile-map inputs")
	flag.BoolVar(&inPlace, "in-place", false, "Rewrite the input file in place")
	flag.BoolVar(&force, "force", false, "Overwrite the output file when it already exists")
	flag.BoolVar(&backupInPlace, "backup-in-place", true, "Create <input>.bak before an in-place rewrite")
	flag.BoolVar(&dryRun, "dry-run", false, "Print migrated YAML to stdout instead of writing a file")
	flag.Parse()

	result, err := cmdhelpers.MigrateEngineProfilesFile(cmdhelpers.EngineProfileMigrationOptions{
		InputPath:       strings.TrimSpace(inputPath),
		OutputPath:      strings.TrimSpace(outputPath),
		RegistrySlugRaw: strings.TrimSpace(registrySlug),
		InPlace:         inPlace,
		Force:           force,
		BackupInPlace:   backupInPlace,
		DryRun:          dryRun,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", warning)
	}

	if dryRun {
		_, _ = os.Stdout.Write(result.OutputYAML)
		return
	}

	fmt.Printf("Input: %s\n", result.InputPath)
	fmt.Printf("Input format: %s\n", result.InputFormat)
	fmt.Printf("Profiles: %d\n", result.ProfileCount)
	if result.WroteFile {
		fmt.Printf("Output: %s\n", result.OutputPath)
	}
	if strings.TrimSpace(result.CreatedBackupPath) != "" {
		fmt.Printf("Backup: %s\n", result.CreatedBackupPath)
	}
}
