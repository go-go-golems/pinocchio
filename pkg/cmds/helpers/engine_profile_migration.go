package helpers

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	embeddingsconfig "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/gemini"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/ollama"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"gopkg.in/yaml.v3"
)

type EngineProfileMigrationOptions struct {
	InputPath       string
	OutputPath      string
	RegistrySlugRaw string
	InPlace         bool
	Force           bool
	BackupInPlace   bool
	DryRun          bool
}

type EngineProfileMigrationResult struct {
	InputPath         string
	OutputPath        string
	InputFormat       string
	ProfileCount      int
	WroteFile         bool
	CreatedBackupPath string
	OutputYAML        []byte
	Warnings          []string
}

type mixedRuntimeSpec struct {
	StepSettingsPatch map[string]any `yaml:"step_settings_patch"`
	SystemPrompt      string         `yaml:"system_prompt"`
	Middlewares       []any          `yaml:"middlewares"`
	Tools             []string       `yaml:"tools"`
}

type mixedProfile struct {
	Slug              string                            `yaml:"slug"`
	DisplayName       string                            `yaml:"display_name"`
	Description       string                            `yaml:"description"`
	Stack             []gepprofiles.EngineProfileRef    `yaml:"stack"`
	InferenceSettings map[string]any                    `yaml:"inference_settings"`
	Runtime           mixedRuntimeSpec                  `yaml:"runtime"`
	Metadata          gepprofiles.EngineProfileMetadata `yaml:"metadata"`
	Extensions        map[string]any                    `yaml:"extensions"`
}

type mixedRegistry struct {
	Slug                     string                       `yaml:"slug"`
	DisplayName              string                       `yaml:"display_name"`
	Description              string                       `yaml:"description"`
	DefaultEngineProfileSlug string                       `yaml:"default_profile_slug"`
	Profiles                 map[string]*mixedProfile     `yaml:"profiles"`
	Metadata                 gepprofiles.RegistryMetadata `yaml:"metadata"`
}

func DetectEngineProfileYAMLFormat(data []byte) string {
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
		return "bundle"
	}
	profilesRaw, hasProfiles := raw["profiles"]
	if !hasProfiles {
		return "legacy-map"
	}
	profilesMap, ok := toStringAnyMap(profilesRaw)
	if !ok {
		return "invalid"
	}
	for _, profileRaw := range profilesMap {
		profileMap, ok := toStringAnyMap(profileRaw)
		if !ok {
			continue
		}
		if _, hasRuntime := profileMap["runtime"]; hasRuntime {
			return "mixed-runtime"
		}
	}
	return "engine-profiles"
}

func MigrateEngineProfilesYAML(data []byte, registrySlugRaw string) (*gepprofiles.EngineProfileRegistry, []string, string, error) {
	format := DetectEngineProfileYAMLFormat(data)
	switch format {
	case "empty":
		return nil, nil, format, fmt.Errorf("profiles YAML is empty")
	case "invalid":
		return nil, nil, format, fmt.Errorf("profiles YAML is invalid")
	case "bundle":
		return nil, nil, format, fmt.Errorf("top-level registries bundles are not supported here; use a single-registry profiles.yaml")
	}

	registrySlug := gepprofiles.MustRegistrySlug("default")
	if strings.TrimSpace(registrySlugRaw) != "" {
		parsed, err := gepprofiles.ParseRegistrySlug(registrySlugRaw)
		if err != nil {
			return nil, nil, format, err
		}
		registrySlug = parsed
	}

	switch format {
	case "engine-profiles":
		registry, err := gepprofiles.DecodeEngineProfileYAMLSingleRegistry(data)
		return registry, nil, format, err
	case "mixed-runtime":
		registry, warnings, err := migrateMixedRuntimeRegistry(data, registrySlug)
		return registry, warnings, format, err
	case "legacy-map":
		registry, warnings, err := migrateLegacyProfileMap(data, registrySlug)
		return registry, warnings, format, err
	default:
		return nil, nil, format, fmt.Errorf("unsupported format %q", format)
	}
}

func MigrateEngineProfilesFile(opts EngineProfileMigrationOptions) (*EngineProfileMigrationResult, error) {
	inputPath := strings.TrimSpace(opts.InputPath)
	if inputPath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user config dir: %w", err)
		}
		inputPath = filepath.Join(configDir, "pinocchio", "profiles.yaml")
	}
	inputPath = filepath.Clean(inputPath)

	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read input profiles file %q: %w", inputPath, err)
	}

	registry, warnings, format, err := MigrateEngineProfilesYAML(raw, opts.RegistrySlugRaw)
	if err != nil {
		return nil, err
	}
	if registry == nil {
		return nil, fmt.Errorf("no engine profile registry produced from %q", inputPath)
	}

	out, err := gepprofiles.EncodeEngineProfileYAMLSingleRegistry(registry)
	if err != nil {
		return nil, fmt.Errorf("encode engine-profile YAML: %w", err)
	}

	outputPath := strings.TrimSpace(opts.OutputPath)
	if opts.InPlace {
		outputPath = inputPath
	} else if outputPath == "" {
		outputPath = inputPath + ".engine-profiles.yaml"
	}
	outputPath = filepath.Clean(outputPath)

	result := &EngineProfileMigrationResult{
		InputPath:    inputPath,
		OutputPath:   outputPath,
		InputFormat:  format,
		ProfileCount: len(registry.Profiles),
		OutputYAML:   out,
		Warnings:     append([]string(nil), warnings...),
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

func migrateMixedRuntimeRegistry(data []byte, fallbackRegistrySlug gepprofiles.RegistrySlug) (*gepprofiles.EngineProfileRegistry, []string, error) {
	var raw mixedRegistry
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil, err
	}
	registrySlug := fallbackRegistrySlug
	if strings.TrimSpace(raw.Slug) != "" {
		parsed, err := gepprofiles.ParseRegistrySlug(raw.Slug)
		if err != nil {
			return nil, nil, err
		}
		registrySlug = parsed
	}
	if registrySlug.IsZero() {
		registrySlug = gepprofiles.MustRegistrySlug("default")
	}
	registry := &gepprofiles.EngineProfileRegistry{
		Slug:        registrySlug,
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		Profiles:    map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{},
		Metadata:    raw.Metadata,
	}
	if strings.TrimSpace(raw.DefaultEngineProfileSlug) != "" {
		slug, err := gepprofiles.ParseEngineProfileSlug(raw.DefaultEngineProfileSlug)
		if err != nil {
			return nil, nil, err
		}
		registry.DefaultEngineProfileSlug = slug
	}

	warnings := []string{}
	keys := make([]string, 0, len(raw.Profiles))
	for key := range raw.Profiles {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		profile, ws, err := migrateMixedProfile(key, raw.Profiles[key])
		if err != nil {
			return nil, nil, err
		}
		warnings = append(warnings, ws...)
		registry.Profiles[profile.Slug] = profile
	}

	if registry.DefaultEngineProfileSlug.IsZero() && len(keys) > 0 {
		if _, ok := registry.Profiles[gepprofiles.MustEngineProfileSlug("default")]; ok {
			registry.DefaultEngineProfileSlug = gepprofiles.MustEngineProfileSlug("default")
		} else {
			registry.DefaultEngineProfileSlug = gepprofiles.MustEngineProfileSlug(keys[0])
		}
	}

	if err := gepprofiles.ValidateRegistry(registry); err != nil {
		return nil, nil, err
	}
	return registry, warnings, nil
}

func migrateMixedProfile(key string, raw *mixedProfile) (*gepprofiles.EngineProfile, []string, error) {
	if raw == nil {
		raw = &mixedProfile{}
	}
	slugRaw := strings.TrimSpace(raw.Slug)
	if slugRaw == "" {
		slugRaw = key
	}
	slug, err := gepprofiles.ParseEngineProfileSlug(slugRaw)
	if err != nil {
		return nil, nil, err
	}

	var finalSettings *aisettings.InferenceSettings
	if len(raw.InferenceSettings) > 0 {
		finalSettings, err = decodeInferenceSettingsMap(raw.InferenceSettings)
		if err != nil {
			return nil, nil, fmt.Errorf("decode inference_settings for profile %q: %w", slug, err)
		}
	}
	if len(raw.Runtime.StepSettingsPatch) > 0 {
		patchSettings, err := inferenceSettingsFromSectionPatch(raw.Runtime.StepSettingsPatch)
		if err != nil {
			return nil, nil, fmt.Errorf("convert runtime.step_settings_patch for profile %q: %w", slug, err)
		}
		if finalSettings == nil {
			finalSettings = patchSettings
		} else {
			finalSettings, err = gepprofiles.MergeInferenceSettings(finalSettings, patchSettings)
			if err != nil {
				return nil, nil, fmt.Errorf("merge inference settings for profile %q: %w", slug, err)
			}
		}
	}

	warnings := []string{}
	if strings.TrimSpace(raw.Runtime.SystemPrompt) != "" {
		warnings = append(warnings, fmt.Sprintf("profile %q: dropped runtime.system_prompt; move prompts to Pinocchio app config or command logic", slug))
	}
	if len(raw.Runtime.Middlewares) > 0 {
		warnings = append(warnings, fmt.Sprintf("profile %q: dropped runtime.middlewares; move middleware selection to Pinocchio app config or code", slug))
	}
	if len(raw.Runtime.Tools) > 0 {
		warnings = append(warnings, fmt.Sprintf("profile %q: dropped runtime.tools; move tool selection to Pinocchio app config or code", slug))
	}

	return &gepprofiles.EngineProfile{
		Slug:              slug,
		DisplayName:       raw.DisplayName,
		Description:       raw.Description,
		Stack:             append([]gepprofiles.EngineProfileRef(nil), raw.Stack...),
		InferenceSettings: finalSettings,
		Metadata:          raw.Metadata,
		Extensions:        cloneMap(raw.Extensions),
	}, warnings, nil
}

func migrateLegacyProfileMap(data []byte, registrySlug gepprofiles.RegistrySlug) (*gepprofiles.EngineProfileRegistry, []string, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil, err
	}
	if len(raw) == 0 {
		return nil, nil, fmt.Errorf("legacy profile map is empty")
	}

	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	profiles := map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{}
	for _, key := range keys {
		slug, err := gepprofiles.ParseEngineProfileSlug(key)
		if err != nil {
			return nil, nil, err
		}
		patchMap, ok := toStringAnyMap(raw[key])
		if !ok {
			return nil, nil, fmt.Errorf("legacy profile %q must map to a section object", key)
		}
		inferenceSettings, err := inferenceSettingsFromSectionPatch(patchMap)
		if err != nil {
			return nil, nil, fmt.Errorf("convert legacy profile %q to inference_settings: %w", key, err)
		}
		profiles[slug] = &gepprofiles.EngineProfile{
			Slug:              slug,
			InferenceSettings: inferenceSettings,
		}
	}

	defaultSlug := gepprofiles.MustEngineProfileSlug(keys[0])
	if _, ok := profiles[gepprofiles.MustEngineProfileSlug("default")]; ok {
		defaultSlug = gepprofiles.MustEngineProfileSlug("default")
	}

	registry := &gepprofiles.EngineProfileRegistry{
		Slug:                     registrySlug,
		DefaultEngineProfileSlug: defaultSlug,
		Profiles:                 profiles,
	}
	if err := gepprofiles.ValidateRegistry(registry); err != nil {
		return nil, nil, err
	}
	return registry, nil, nil
}

func decodeInferenceSettingsMap(raw map[string]any) (*aisettings.InferenceSettings, error) {
	ss, err := aisettings.NewInferenceSettings()
	if err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, ss); err != nil {
		return nil, err
	}
	return ss, nil
}

func inferenceSettingsFromSectionPatch(raw map[string]any) (*aisettings.InferenceSettings, error) {
	sections, err := migrationInferenceSections()
	if err != nil {
		return nil, err
	}
	schema_ := schema.NewSchema(schema.WithSections(sections...))
	parsed := values.New()
	if err := schema_.ForEachE(func(_ string, section schema.Section) error {
		parsed.GetOrCreate(section)
		return nil
	}); err != nil {
		return nil, err
	}
	normalized, err := normalizeSectionPatchMap(raw)
	if err != nil {
		return nil, err
	}
	if err := sources.Execute(schema_, parsed, sources.FromMap(normalized)); err != nil {
		return nil, err
	}
	return aisettings.NewInferenceSettingsFromParsedValues(parsed)
}

func migrationInferenceSections() ([]schema.Section, error) {
	chatSection, err := aisettings.NewChatValueSection()
	if err != nil {
		return nil, err
	}
	clientSection, err := aisettings.NewClientValueSection()
	if err != nil {
		return nil, err
	}
	claudeSection, err := claude.NewValueSection()
	if err != nil {
		return nil, err
	}
	geminiSection, err := gemini.NewValueSection()
	if err != nil {
		return nil, err
	}
	openaiSection, err := openai.NewValueSection()
	if err != nil {
		return nil, err
	}
	ollamaSection, err := ollama.NewValueSection()
	if err != nil {
		return nil, err
	}
	embeddingsSection, err := embeddingsconfig.NewEmbeddingsValueSection()
	if err != nil {
		return nil, err
	}
	inferenceSection, err := aisettings.NewInferenceValueSection()
	if err != nil {
		return nil, err
	}
	return []schema.Section{
		chatSection,
		clientSection,
		claudeSection,
		geminiSection,
		openaiSection,
		ollamaSection,
		embeddingsSection,
		inferenceSection,
	}, nil
}

func normalizeSectionPatchMap(raw map[string]any) (map[string]map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := map[string]map[string]any{}
	for sectionSlugRaw, sectionRaw := range raw {
		sectionSlug := strings.TrimSpace(sectionSlugRaw)
		if sectionSlug == "" {
			return nil, fmt.Errorf("section slug cannot be empty")
		}
		sectionMap, ok := toStringAnyMap(sectionRaw)
		if !ok {
			return nil, fmt.Errorf("section patch %q must be an object", sectionSlug)
		}
		fieldMap := map[string]any{}
		for fieldNameRaw, value := range sectionMap {
			fieldName := strings.TrimSpace(fieldNameRaw)
			if fieldName == "" {
				return nil, fmt.Errorf("field name in section %q cannot be empty", sectionSlug)
			}
			fieldMap[fieldName] = value
		}
		out[sectionSlug] = fieldMap
	}
	return out, nil
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory for %q: %w", path, err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, mode); err != nil {
		return fmt.Errorf("write temporary output file %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %q to %q: %w", tmpPath, path, err)
	}
	return nil
}

func toStringAnyMap(in any) (map[string]any, bool) {
	switch v := in.(type) {
	case map[string]any:
		return v, true
	default:
		return nil, false
	}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
