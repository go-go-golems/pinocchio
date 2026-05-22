package profiles

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	geppettobootstrap "github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	aistepssettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	"gopkg.in/yaml.v3"
)

const (
	VerbosityDefault  = "default"
	VerbosityDetailed = "detailed"
	VerbosityFull     = "full"
)

type ListCommand struct {
	*cmds.CommandDescription
}

type ListSettings struct {
	Verbosity string `glazed:"verbosity"`
}

var _ cmds.GlazeCommand = (*ListCommand)(nil)

func NewListCommand() (*ListCommand, error) {
	glazedSection, err := settings.NewGlazedSection()
	if err != nil {
		return nil, err
	}
	commandSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}
	profileSettingsSection, err := geppettosections.NewProfileSettingsSection()
	if err != nil {
		return nil, err
	}

	return &ListCommand{
		CommandDescription: cmds.NewCommandDescription(
			"list",
			cmds.WithShort("List Pinocchio engine profiles"),
			cmds.WithLong(`List profiles from Pinocchio's configured profile registry chain.

Examples:
  pinocchio profiles list
  pinocchio profiles list --profile researcher
  pinocchio profiles list --profile-registries ./profiles.yaml --verbosity detailed --output json
  pinocchio profiles list --verbosity full --fields registry,profile,override_paths,effective_settings_json
`),
			cmds.WithFlags(
				fields.New(
					"verbosity",
					fields.TypeString,
					fields.WithDefault(VerbosityDefault),
					fields.WithHelp("Amount of profile detail to include: default, detailed, full"),
				),
			),
			cmds.WithSections(glazedSection, commandSettingsSection, profileSettingsSection),
		),
	}, nil
}

func (c *ListCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	s := &ListSettings{Verbosity: VerbosityDefault}
	if parsedLayers != nil {
		if err := parsedLayers.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
			return fmt.Errorf("decode profiles list settings: %w", err)
		}
	}
	s.Verbosity = strings.ToLower(strings.TrimSpace(s.Verbosity))
	if s.Verbosity == "" {
		s.Verbosity = VerbosityDefault
	}
	if err := validateVerbosity(s.Verbosity); err != nil {
		return err
	}

	runtime, err := profilebootstrap.ResolveCLIProfileRuntime(ctx, parsedLayers)
	if err != nil {
		return fmt.Errorf("resolve profile runtime: %w", err)
	}
	if runtime != nil && runtime.Close != nil {
		defer runtime.Close()
	}
	if runtime == nil || runtime.ProfileRegistryChain == nil || runtime.ProfileRegistryChain.Registry == nil {
		return nil
	}

	report, err := buildReportFromRuntime(ctx, runtime, s.Verbosity)
	if err != nil {
		return err
	}

	registry := runtime.ProfileRegistryChain.Registry
	registrySummaries := registrySummariesBySlug(report)
	selectedRegistry, selectedProfile := selectedProfileRef(report)
	for _, summary := range report.Profiles {
		profile, err := registry.GetEngineProfile(ctx, gepprofiles.RegistrySlug(summary.Registry), gepprofiles.EngineProfileSlug(summary.Slug))
		if err != nil {
			return fmt.Errorf("load profile %s/%s: %w", summary.Registry, summary.Slug, err)
		}
		resolved, _ := resolveProfile(ctx, registry, summary.Registry, summary.Slug)
		row := buildProfileRow(report, registrySummaries, summary, profile, resolved, selectedRegistry, selectedProfile, s.Verbosity)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

func validateVerbosity(v string) error {
	switch v {
	case VerbosityDefault, VerbosityDetailed, VerbosityFull:
		return nil
	default:
		return fmt.Errorf("invalid verbosity %q: expected default, detailed, or full", v)
	}
}

func buildReportFromRuntime(ctx context.Context, runtime *profilebootstrap.ResolvedCLIProfileRuntime, verbosity string) (*profilebootstrap.ProfileRegistryReport, error) {
	chain := runtime.ProfileRegistryChain
	defaultProfileSlug := gepprofiles.EngineProfileSlug("")
	if !chain.DefaultRegistrySlug.IsZero() && chain.DefaultProfileResolve.EngineProfileSlug.IsZero() {
		reg, regErr := chain.Registry.GetRegistry(ctx, chain.DefaultRegistrySlug)
		if regErr == nil && reg != nil && !reg.DefaultEngineProfileSlug.IsZero() {
			defaultProfileSlug = reg.DefaultEngineProfileSlug
		}
	}

	includeResolution := verbosity == VerbosityDetailed || verbosity == VerbosityFull
	report, err := geppettobootstrap.BuildProfileRegistryReportFromRegistry(ctx, geppettobootstrap.ProfileRegistryReportInput{
		SourceEntries:       runtime.ProfileSettings.ProfileRegistries,
		Registry:            chain.Registry,
		DefaultRegistrySlug: chain.DefaultRegistrySlug,
		DefaultProfileSlug:  defaultProfileSlug,
		ResolveInput:        chain.DefaultProfileResolve,
	}, geppettobootstrap.ProfileRegistryReportOptions{
		IncludeResolution:     includeResolution,
		IncludeMergedSettings: verbosity == VerbosityFull,
		RedactSecrets:         true,
	})
	if err != nil {
		return nil, fmt.Errorf("build profile report: %w", err)
	}
	return report, nil
}

func registrySummariesBySlug(report *profilebootstrap.ProfileRegistryReport) map[string]geppettobootstrap.ProfileRegistrySummaryReport {
	ret := map[string]geppettobootstrap.ProfileRegistrySummaryReport{}
	if report == nil {
		return ret
	}
	for _, r := range report.Registries {
		ret[r.Slug] = r
	}
	return ret
}

func selectedProfileRef(report *profilebootstrap.ProfileRegistryReport) (string, string) {
	if report == nil {
		return "", ""
	}
	registry := report.SelectedRegistry
	profile := report.SelectedProfile
	if registry == "" {
		registry = report.DefaultRegistry
	}
	if profile == "" {
		profile = report.DefaultProfile
	}
	return registry, profile
}

func resolveProfile(ctx context.Context, registry gepprofiles.Registry, registrySlug string, profileSlug string) (*gepprofiles.ResolvedEngineProfile, error) {
	if registry == nil || strings.TrimSpace(registrySlug) == "" || strings.TrimSpace(profileSlug) == "" {
		return nil, nil
	}
	resolved, err := registry.ResolveEngineProfile(ctx, gepprofiles.ResolveInput{
		RegistrySlug:      gepprofiles.RegistrySlug(registrySlug),
		EngineProfileSlug: gepprofiles.EngineProfileSlug(profileSlug),
	})
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func buildProfileRow(
	report *profilebootstrap.ProfileRegistryReport,
	registries map[string]geppettobootstrap.ProfileRegistrySummaryReport,
	summary geppettobootstrap.ProfileSummaryReport,
	profile *gepprofiles.EngineProfile,
	resolved *gepprofiles.ResolvedEngineProfile,
	selectedRegistry string,
	selectedProfile string,
	verbosity string,
) types.Row {
	overrides := extractOverrideSummary(profile)
	effective := extractEffectiveSummary(resolved)
	selected := summary.IsSelected || (selectedProfile != "" && summary.Slug == selectedProfile && (selectedRegistry == "" || summary.Registry == selectedRegistry))
	row := types.NewRow(
		types.MRP("selected", selected),
		types.MRP("default", summary.IsDefault),
		types.MRP("registry", summary.Registry),
		types.MRP("profile", summary.Slug),
		types.MRP("display_name", summary.DisplayName),
		types.MRP("effective_chat_engine", firstNonEmpty(effective.ChatEngine, summary.Model)),
		types.MRP("effective_chat_api_type", firstNonEmpty(effective.ChatAPIType, summary.APIType)),
		types.MRP("reasoning_effort", effective.ReasoningEffort),
		types.MRP("description", summary.Description),
	)

	if verbosity == VerbosityDetailed || verbosity == VerbosityFull {
		reg := registries[summary.Registry]
		row.Set("version", summary.Version)
		row.Set("source", summary.Source)
		row.Set("registry_default_profile", reg.DefaultProfile)
		row.Set("registry_is_default", reg.IsDefault)
		row.Set("registry_profile_count", reg.ProfileCount)
		row.Set("profile_ref", summary.Registry+"/"+summary.Slug)
		row.Set("override_count", len(overrides.Paths))
		row.Set("override_paths", strings.Join(overrides.Paths, ","))
		row.Set("override_chat_engine", overrides.ChatEngine)
		row.Set("override_chat_api_type", overrides.ChatAPIType)
		row.Set("override_chat_temperature", overrides.ChatTemperature)
		row.Set("override_chat_top_p", overrides.ChatTopP)
		row.Set("override_chat_max_response_tokens", overrides.ChatMaxResponseTokens)
		row.Set("override_inference_reasoning_effort", overrides.ReasoningEffort)
		row.Set("override_inference_reasoning_summary", overrides.ReasoningSummary)
		row.Set("override_inference_thinking_budget", overrides.ThinkingBudget)
		row.Set("override_inference_thinking_type", overrides.ThinkingType)
		row.Set("override_inference_temperature", overrides.InferenceTemperature)
		row.Set("override_inference_top_p", overrides.InferenceTopP)
		row.Set("override_inference_max_response_tokens", overrides.InferenceMaxResponseTokens)
		row.Set("override_model_reasoning", overrides.ModelReasoning)
		row.Set("override_model_context_window", overrides.ModelContextWindow)
		row.Set("override_model_max_output_tokens", overrides.ModelMaxOutputTokens)
		row.Set("effective_reasoning_effort", effective.ReasoningEffort)
		row.Set("effective_reasoning_summary", effective.ReasoningSummary)
		row.Set("effective_thinking_budget", effective.ThinkingBudget)
		row.Set("effective_temperature", firstNonNil(effective.InferenceTemperature, effective.ChatTemperature))
		row.Set("effective_top_p", firstNonNil(effective.InferenceTopP, effective.ChatTopP))
		row.Set("effective_max_response_tokens", firstNonNil(effective.InferenceMaxResponseTokens, effective.ChatMaxResponseTokens))
	}

	if verbosity == VerbosityFull {
		row.Set("default_registry", valueOrEmpty(report, func(r *profilebootstrap.ProfileRegistryReport) string { return r.DefaultRegistry }))
		row.Set("default_profile", valueOrEmpty(report, func(r *profilebootstrap.ProfileRegistryReport) string { return r.DefaultProfile }))
		row.Set("selected_registry", valueOrEmpty(report, func(r *profilebootstrap.ProfileRegistryReport) string { return r.SelectedRegistry }))
		row.Set("selected_profile", valueOrEmpty(report, func(r *profilebootstrap.ProfileRegistryReport) string { return r.SelectedProfile }))
		if resolved != nil {
			row.Set("resolved_registry", resolved.RegistrySlug.String())
			row.Set("resolved_profile", resolved.EngineProfileSlug.String())
			row.Set("resolution_lineage", lineageStrings(resolved.StackLineage))
			row.Set("resolution_metadata", geppettobootstrap.RedactProfileSecrets(resolved.Metadata))
		}
		row.Set("override_settings_json", overrides.SettingsJSON)
		row.Set("effective_settings_json", effective.SettingsJSON)
		if report != nil && report.Resolution != nil && report.Resolution.Registry == summary.Registry && report.Resolution.Profile == summary.Slug {
			row.Set("merged_inference_settings", report.Resolution.InferenceSettings)
		}
	}

	return row
}

type settingsSummary struct {
	Paths                      []string
	SettingsJSON               string
	ChatEngine                 string
	ChatAPIType                string
	ChatTemperature            any
	ChatTopP                   any
	ChatMaxResponseTokens      any
	ReasoningEffort            string
	ReasoningSummary           string
	ThinkingBudget             any
	ThinkingType               string
	InferenceTemperature       any
	InferenceTopP              any
	InferenceMaxResponseTokens any
	ModelReasoning             any
	ModelContextWindow         any
	ModelMaxOutputTokens       any
}

func extractOverrideSummary(profile *gepprofiles.EngineProfile) settingsSummary {
	if profile == nil {
		return settingsSummary{}
	}
	return summarizeSettings(profile.InferenceSettings)
}

func extractEffectiveSummary(resolved *gepprofiles.ResolvedEngineProfile) settingsSummary {
	if resolved == nil {
		return settingsSummary{}
	}
	return summarizeSettings(resolved.InferenceSettings)
}

func summarizeSettings(s *aistepssettings.InferenceSettings) settingsSummary {
	ret := settingsSummary{}
	if s == nil {
		return ret
	}
	if m := settingsToMap(s); len(m) > 0 {
		ret.Paths = flattenPaths(m)
		ret.SettingsJSON = mustJSON(geppettobootstrap.RedactProfileSecrets(m))
	}
	if s.Chat != nil {
		if s.Chat.Engine != nil {
			ret.ChatEngine = *s.Chat.Engine
		}
		if s.Chat.ApiType != nil {
			ret.ChatAPIType = string(*s.Chat.ApiType)
		}
		if s.Chat.Temperature != nil {
			ret.ChatTemperature = *s.Chat.Temperature
		}
		if s.Chat.TopP != nil {
			ret.ChatTopP = *s.Chat.TopP
		}
		if s.Chat.MaxResponseTokens != nil {
			ret.ChatMaxResponseTokens = *s.Chat.MaxResponseTokens
		}
	}
	if s.Inference != nil {
		if s.Inference.ReasoningEffort != nil {
			ret.ReasoningEffort = *s.Inference.ReasoningEffort
		}
		if s.Inference.ReasoningSummary != nil {
			ret.ReasoningSummary = *s.Inference.ReasoningSummary
		}
		if s.Inference.ThinkingBudget != nil {
			ret.ThinkingBudget = *s.Inference.ThinkingBudget
		}
		if s.Inference.ThinkingType != nil {
			ret.ThinkingType = *s.Inference.ThinkingType
		}
		if s.Inference.Temperature != nil {
			ret.InferenceTemperature = *s.Inference.Temperature
		}
		if s.Inference.TopP != nil {
			ret.InferenceTopP = *s.Inference.TopP
		}
		if s.Inference.MaxResponseTokens != nil {
			ret.InferenceMaxResponseTokens = *s.Inference.MaxResponseTokens
		}
	}
	if s.ModelInfo != nil {
		if s.ModelInfo.Reasoning != nil {
			ret.ModelReasoning = *s.ModelInfo.Reasoning
		}
		if s.ModelInfo.ContextWindow != nil {
			ret.ModelContextWindow = *s.ModelInfo.ContextWindow
		}
		if s.ModelInfo.MaxOutputTokens != nil {
			ret.ModelMaxOutputTokens = *s.ModelInfo.MaxOutputTokens
		}
	}
	return ret
}

func settingsToMap(s *aistepssettings.InferenceSettings) map[string]any {
	b, err := yaml.Marshal(s)
	if err != nil {
		return nil
	}
	var ret map[string]any
	if err := yaml.Unmarshal(b, &ret); err != nil {
		return nil
	}
	pruneEmpty(ret)
	return ret
}

func pruneEmpty(m map[string]any) bool {
	for k, v := range m {
		switch vv := v.(type) {
		case map[string]any:
			if pruneEmpty(vv) {
				delete(m, k)
			}
		case []any:
			if len(vv) == 0 {
				delete(m, k)
			}
		case string:
			if vv == "" {
				delete(m, k)
			}
		case nil:
			delete(m, k)
		}
	}
	return len(m) == 0
}

func flattenPaths(m map[string]any) []string {
	paths := make([]string, 0)
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch vv := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(vv))
			for k := range vv {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				next := k
				if prefix != "" {
					next = prefix + "." + k
				}
				walk(next, vv[k])
			}
		default:
			if prefix != "" {
				paths = append(paths, prefix)
			}
		}
	}
	walk("", m)
	return paths
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func lineageStrings(lineage []gepprofiles.ResolvedProfileStackEntry) []string {
	ret := make([]string, 0, len(lineage))
	for _, entry := range lineage {
		ref := entry.RegistrySlug.String() + "/" + entry.EngineProfileSlug.String()
		if entry.Version != 0 {
			ref += fmt.Sprintf("@%d", entry.Version)
		}
		if strings.TrimSpace(entry.Source) != "" {
			ref += " (" + strings.TrimSpace(entry.Source) + ")"
		}
		ret = append(ret, ref)
	}
	return ret
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func valueOrEmpty(report *profilebootstrap.ProfileRegistryReport, f func(*profilebootstrap.ProfileRegistryReport) string) string {
	if report == nil {
		return ""
	}
	return f(report)
}
