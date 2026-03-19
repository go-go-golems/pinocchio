package cmds

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	clay_profiles "github.com/go-go-golems/clay/pkg/cmds/profiles"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"gopkg.in/yaml.v3"
)

type MigrateLegacyProfilesSettings struct {
	InputPath       string `glazed:"input"`
	OutputPath      string `glazed:"output"`
	Registry        string `glazed:"registry"`
	InPlace         bool   `glazed:"in-place"`
	Force           bool   `glazed:"force"`
	BackupInPlace   bool   `glazed:"backup-in-place"`
	DryRun          bool   `glazed:"dry-run"`
	SkipIfNotLegacy bool   `glazed:"skip-if-not-legacy"`
}

type MigrateLegacyProfilesCommand struct {
	*cmds.CommandDescription
}

type LegacyProfilesMigrationOptions struct {
	InputPath       string
	OutputPath      string
	RegistrySlugRaw string
	InPlace         bool
	Force           bool
	BackupInPlace   bool
	DryRun          bool
	SkipIfNotLegacy bool
}

type LegacyProfilesMigrationResult struct {
	InputPath         string
	OutputPath        string
	InputFormat       string
	RegistryCount     int
	ProfileCount      int
	WroteFile         bool
	CreatedBackupPath string
	OutputYAML        []byte
}

var _ cmds.WriterCommand = &MigrateLegacyProfilesCommand{}

func NewMigrateLegacyProfilesCommand() (*MigrateLegacyProfilesCommand, error) {
	return &MigrateLegacyProfilesCommand{
		CommandDescription: cmds.NewCommandDescription(
			"migrate-legacy",
			cmds.WithShort("Migrate legacy profiles.yaml map format to runtime single-registry YAML"),
			cmds.WithLong(`Convert legacy Pinocchio profiles format (profile -> layer settings map)
to runtime single-registry profile YAML (slug + profiles) for registry-first runtime flows.`),
			cmds.WithFlags(
				fields.New(
					"input",
					fields.TypeString,
					fields.WithHelp("Input profiles YAML path (defaults to pinocchio profiles path)"),
					fields.WithDefault(""),
				),
				fields.New(
					"output",
					fields.TypeString,
					fields.WithHelp("Output runtime registry YAML path (defaults to <input>.runtime.yaml unless --in-place)"),
					fields.WithDefault(""),
				),
				fields.New(
					"registry",
					fields.TypeString,
					fields.WithHelp("Default registry slug used when converting legacy map input"),
					fields.WithDefault("default"),
				),
				fields.New(
					"in-place",
					fields.TypeBool,
					fields.WithHelp("Write migration output back to the input file"),
					fields.WithDefault(false),
				),
				fields.New(
					"backup-in-place",
					fields.TypeBool,
					fields.WithHelp("When --in-place is set, create <input>.bak before overwrite"),
					fields.WithDefault(true),
				),
				fields.New(
					"force",
					fields.TypeBool,
					fields.WithHelp("Overwrite output file when it already exists"),
					fields.WithDefault(false),
				),
				fields.New(
					"dry-run",
					fields.TypeBool,
					fields.WithHelp("Print converted YAML to stdout without writing files"),
					fields.WithDefault(false),
				),
				fields.New(
					"skip-if-not-legacy",
					fields.TypeBool,
					fields.WithHelp("No-op when input is already single-registry runtime format"),
					fields.WithDefault(false),
				),
			),
		),
	}, nil
}

func (c *MigrateLegacyProfilesCommand) RunIntoWriter(
	_ context.Context,
	parsedLayers *values.Values,
	w io.Writer,
) error {
	settings := &MigrateLegacyProfilesSettings{}
	if err := parsedLayers.DecodeSectionInto(values.DefaultSlug, settings); err != nil {
		return fmt.Errorf("initialize settings: %w", err)
	}

	result, err := MigrateLegacyProfilesFile(LegacyProfilesMigrationOptions{
		InputPath:       strings.TrimSpace(settings.InputPath),
		OutputPath:      strings.TrimSpace(settings.OutputPath),
		RegistrySlugRaw: strings.TrimSpace(settings.Registry),
		InPlace:         settings.InPlace,
		Force:           settings.Force,
		BackupInPlace:   settings.BackupInPlace,
		DryRun:          settings.DryRun,
		SkipIfNotLegacy: settings.SkipIfNotLegacy,
	})
	if err != nil {
		return err
	}

	if result == nil {
		return nil
	}

	if settings.DryRun {
		if _, err := w.Write(result.OutputYAML); err != nil {
			return fmt.Errorf("write dry-run output: %w", err)
		}
		return nil
	}

	fmt.Fprintf(w, "Input: %s\n", result.InputPath)
	fmt.Fprintf(w, "Input format: %s\n", result.InputFormat)
	if result.WroteFile {
		fmt.Fprintf(w, "Output: %s\n", result.OutputPath)
	} else {
		fmt.Fprintln(w, "No output written")
	}
	fmt.Fprintf(w, "Registries: %d\n", result.RegistryCount)
	fmt.Fprintf(w, "Profiles: %d\n", result.ProfileCount)
	if strings.TrimSpace(result.CreatedBackupPath) != "" {
		fmt.Fprintf(w, "Backup: %s\n", result.CreatedBackupPath)
	}
	return nil
}

func MigrateLegacyProfilesFile(opts LegacyProfilesMigrationOptions) (*LegacyProfilesMigrationResult, error) {
	inputPath := strings.TrimSpace(opts.InputPath)
	if inputPath == "" {
		var err error
		inputPath, err = clay_profiles.GetProfilesPathForApp("pinocchio")
		if err != nil {
			return nil, fmt.Errorf("resolve default pinocchio profiles path: %w", err)
		}
	}
	inputPath = filepath.Clean(inputPath)

	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read input profiles file %q: %w", inputPath, err)
	}

	registrySlug := gepprofiles.MustRegistrySlug("default")
	if strings.TrimSpace(opts.RegistrySlugRaw) != "" {
		parsed, err := gepprofiles.ParseRegistrySlug(opts.RegistrySlugRaw)
		if err != nil {
			return nil, fmt.Errorf("parse registry slug %q: %w", opts.RegistrySlugRaw, err)
		}
		registrySlug = parsed
	}

	inputFormat := detectProfilesYAMLFormat(raw)
	if opts.SkipIfNotLegacy && (inputFormat == "canonical-registries" || inputFormat == "single-registry") {
		return &LegacyProfilesMigrationResult{
			InputPath:   inputPath,
			OutputPath:  "",
			InputFormat: inputFormat,
			WroteFile:   false,
			OutputYAML:  nil,
		}, nil
	}
	if inputFormat == "empty" {
		return nil, fmt.Errorf("input profiles file %q is empty", inputPath)
	}

	var runtimeRegistry *gepprofiles.ProfileRegistry
	switch inputFormat {
	case "legacy-map":
		runtimeRegistry, err = convertLegacyProfilesMapToRuntimeRegistry(raw, registrySlug)
		if err != nil {
			return nil, fmt.Errorf("decode legacy profiles YAML: %w", err)
		}
	case "single-registry":
		runtimeRegistry, err = gepprofiles.DecodeRuntimeYAMLSingleRegistry(raw)
		if err != nil {
			return nil, fmt.Errorf("decode single-registry runtime YAML: %w", err)
		}
	case "canonical-registries":
		return nil, fmt.Errorf("runtime bundle format (top-level registries) is deprecated; provide or convert to a single-registry runtime YAML file")
	default:
		return nil, fmt.Errorf("unsupported profiles YAML format %q", inputFormat)
	}
	if runtimeRegistry == nil {
		return nil, fmt.Errorf("no runtime registry decoded from %q", inputPath)
	}
	out, err := encodeRuntimeRegistryYAML(runtimeRegistry)
	if err != nil {
		return nil, fmt.Errorf("encode runtime registry YAML: %w", err)
	}

	profileCount := len(runtimeRegistry.Profiles)

	outputPath := strings.TrimSpace(opts.OutputPath)
	if opts.InPlace {
		outputPath = inputPath
	} else if outputPath == "" {
		outputPath = inputPath + ".runtime.yaml"
	}
	outputPath = filepath.Clean(outputPath)

	result := &LegacyProfilesMigrationResult{
		InputPath:     inputPath,
		OutputPath:    outputPath,
		InputFormat:   inputFormat,
		RegistryCount: 1,
		ProfileCount:  profileCount,
		OutputYAML:    out,
	}
	if opts.DryRun {
		return result, nil
	}

	if !opts.InPlace {
		if _, err := os.Stat(outputPath); err == nil && !opts.Force {
			return nil, fmt.Errorf("output file already exists: %s (use --force)", outputPath)
		}
	}

	if opts.InPlace && opts.BackupInPlace {
		backupPath := inputPath + ".bak"
		// #nosec G703 -- backup path is intentionally derived from the user-selected input file path in this local CLI command.
		if err := os.WriteFile(backupPath, raw, 0o644); err != nil {
			return nil, fmt.Errorf("write backup file %q: %w", backupPath, err)
		}
		result.CreatedBackupPath = backupPath
	}

	if err := writeFileAtomically(outputPath, out, 0o644); err != nil {
		return nil, err
	}
	result.WroteFile = true
	return result, nil
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory for %q: %w", path, err)
	}
	tmpPath := path + ".tmp"
	// #nosec G703 -- temporary path is intentionally derived from the validated target output path for atomic replacement.
	if err := os.WriteFile(tmpPath, data, mode); err != nil {
		return fmt.Errorf("write temporary output file %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %q to %q: %w", tmpPath, path, err)
	}
	return nil
}

func detectProfilesYAMLFormat(data []byte) string {
	if len(bytes.TrimSpace(data)) == 0 {
		return "empty"
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "invalid"
	}
	if len(raw) == 0 {
		return "empty"
	}
	if _, ok := raw["registries"]; ok {
		return "canonical-registries"
	}
	if _, ok := raw["profiles"]; ok {
		return "single-registry"
	}
	if _, ok := raw["slug"]; ok {
		return "single-registry"
	}
	if _, ok := raw["default_profile_slug"]; ok {
		return "single-registry"
	}
	return "legacy-map"
}

func convertLegacyProfilesMapToRuntimeRegistry(data []byte, registrySlug gepprofiles.RegistrySlug) (*gepprofiles.ProfileRegistry, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("legacy profile map is empty")
	}
	if registrySlug.IsZero() {
		registrySlug = gepprofiles.MustRegistrySlug("default")
	}

	profileKeyOrder := make([]string, 0, len(raw))
	for key := range raw {
		profileKeyOrder = append(profileKeyOrder, key)
	}
	sort.Strings(profileKeyOrder)

	profiles := map[gepprofiles.ProfileSlug]*gepprofiles.Profile{}
	for _, profileKey := range profileKeyOrder {
		profileSlug, err := gepprofiles.ParseProfileSlug(profileKey)
		if err != nil {
			return nil, err
		}
		sectionPatch, ok := raw[profileKey].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("legacy profile %q must map to section settings object", profileKey)
		}
		profiles[profileSlug] = &gepprofiles.Profile{
			Slug: profileSlug,
			Runtime: gepprofiles.RuntimeSpec{
				StepSettingsPatch: cloneStringAnyMap(sectionPatch),
			},
		}
	}

	defaultProfile := gepprofiles.ProfileSlug("")
	if _, ok := profiles[gepprofiles.MustProfileSlug("default")]; ok {
		defaultProfile = gepprofiles.MustProfileSlug("default")
	} else if len(profileKeyOrder) > 0 {
		defaultProfile = gepprofiles.MustProfileSlug(profileKeyOrder[0])
	}

	registry := &gepprofiles.ProfileRegistry{
		Slug:               registrySlug,
		DefaultProfileSlug: defaultProfile,
		Profiles:           profiles,
	}
	if err := gepprofiles.ValidateRegistry(registry); err != nil {
		return nil, err
	}
	return registry, nil
}

type runtimeRegistryYAML struct {
	Slug        gepprofiles.RegistrySlug                         `yaml:"slug"`
	DisplayName string                                           `yaml:"display_name,omitempty"`
	Description string                                           `yaml:"description,omitempty"`
	Profiles    map[gepprofiles.ProfileSlug]*gepprofiles.Profile `yaml:"profiles,omitempty"`
	Metadata    gepprofiles.RegistryMetadata                     `yaml:"metadata,omitempty"`
}

func encodeRuntimeRegistryYAML(registry *gepprofiles.ProfileRegistry) ([]byte, error) {
	if registry == nil {
		return nil, fmt.Errorf("runtime registry is nil")
	}
	clone := registry.Clone()
	if clone == nil {
		return nil, fmt.Errorf("runtime registry clone is nil")
	}
	if err := gepprofiles.ValidateRegistry(clone); err != nil {
		return nil, err
	}
	for profileSlug, profile := range clone.Profiles {
		if profile == nil {
			continue
		}
		if profile.Slug.IsZero() {
			profile.Slug = profileSlug
		}
	}
	out := runtimeRegistryYAML{
		Slug:        clone.Slug,
		DisplayName: clone.DisplayName,
		Description: clone.Description,
		Profiles:    clone.Profiles,
		Metadata:    clone.Metadata,
	}
	return yaml.Marshal(out)
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(in any) any {
	switch v := in.(type) {
	case map[string]any:
		return cloneStringAnyMap(v)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, cloneAny(item))
		}
		return out
	default:
		return in
	}
}
