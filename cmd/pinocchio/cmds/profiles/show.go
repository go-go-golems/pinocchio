package profiles

import (
	"context"
	"fmt"
	"strings"

	geppettobootstrap "github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

type ShowCommand struct {
	*cmds.CommandDescription
}

type ShowSettings struct {
	Profile   string `glazed:"profile-ref"`
	Registry  string `glazed:"registry"`
	Verbosity string `glazed:"verbosity"`
}

var _ cmds.GlazeCommand = (*ShowCommand)(nil)

func NewShowCommand() (*ShowCommand, error) {
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

	return &ShowCommand{
		CommandDescription: cmds.NewCommandDescription(
			"show",
			cmds.WithShort("Show one Pinocchio engine profile"),
			cmds.WithLong(`Show raw overrides and effective resolved settings for one profile.

The profile argument can be a plain profile slug or registry/profile. If omitted,
the selected profile is shown.

Examples:
  pinocchio profiles show
  pinocchio profiles show mini
  pinocchio profiles show workspace/mini --verbosity full --output json
  pinocchio profiles show mini --registry workspace --fields profile,override_paths,effective_settings_json
`),
			cmds.WithFlags(
				fields.New(
					"registry",
					fields.TypeString,
					fields.WithHelp("Registry slug for an unqualified profile argument"),
					fields.WithDefault(""),
				),
				fields.New(
					"verbosity",
					fields.TypeString,
					fields.WithDefault(VerbosityDetailed),
					fields.WithHelp("Amount of profile detail to include: default, detailed, full"),
				),
			),
			cmds.WithArguments(
				fields.New(
					"profile-ref",
					fields.TypeString,
					fields.WithHelp("Profile slug or registry/profile; omitted means selected profile"),
					fields.WithDefault(""),
				),
			),
			cmds.WithSections(glazedSection, commandSettingsSection, profileSettingsSection),
		),
	}, nil
}

func (c *ShowCommand) RunIntoGlazeProcessor(ctx context.Context, parsedLayers *values.Values, gp middlewares.Processor) error {
	s := &ShowSettings{Verbosity: VerbosityDetailed}
	if parsedLayers != nil {
		if err := parsedLayers.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
			return fmt.Errorf("decode profiles show settings: %w", err)
		}
	}
	s.Verbosity = strings.ToLower(strings.TrimSpace(s.Verbosity))
	if s.Verbosity == "" {
		s.Verbosity = VerbosityDetailed
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
		return fmt.Errorf("no profile registry configured")
	}

	report, err := buildReportFromRuntime(ctx, runtime, s.Verbosity)
	if err != nil {
		return err
	}
	registrySlug, profileSlug, err := resolveShowTarget(report, s.Registry, s.Profile)
	if err != nil {
		return err
	}

	registry := runtime.ProfileRegistryChain.Registry
	profile, err := registry.GetEngineProfile(ctx, gepprofiles.RegistrySlug(registrySlug), gepprofiles.EngineProfileSlug(profileSlug))
	if err != nil {
		return fmt.Errorf("load profile %s/%s: %w", registrySlug, profileSlug, err)
	}
	resolved, err := resolveProfile(ctx, registry, registrySlug, profileSlug)
	if err != nil {
		return fmt.Errorf("resolve profile %s/%s: %w", registrySlug, profileSlug, err)
	}

	summary, err := profileSummaryFor(report, registrySlug, profileSlug, profile)
	if err != nil {
		return err
	}
	selectedRegistry, selectedProfile := selectedProfileRef(report)
	row := buildProfileRow(report, registrySummariesBySlug(report), summary, profile, resolved, selectedRegistry, selectedProfile, s.Verbosity)
	return gp.AddRow(ctx, row)
}

func resolveShowTarget(report *profilebootstrap.ProfileRegistryReport, registryFlag string, profileRef string) (string, string, error) {
	registryFlag = strings.TrimSpace(registryFlag)
	profileRef = strings.TrimSpace(profileRef)
	if strings.Contains(profileRef, "/") {
		parts := strings.SplitN(profileRef, "/", 2)
		registryFlag = strings.TrimSpace(parts[0])
		profileRef = strings.TrimSpace(parts[1])
	}
	selectedRegistry, selectedProfile := selectedProfileRef(report)
	if registryFlag == "" {
		registryFlag = selectedRegistry
	}
	if profileRef == "" {
		profileRef = selectedProfile
	}
	if registryFlag == "" {
		return "", "", fmt.Errorf("profiles show requires a registry; pass --registry or configure/select a default registry")
	}
	if profileRef == "" {
		return "", "", fmt.Errorf("profiles show requires a profile argument or selected/default profile")
	}
	return registryFlag, profileRef, nil
}

func profileSummaryFor(report *profilebootstrap.ProfileRegistryReport, registrySlug string, profileSlug string, profile *gepprofiles.EngineProfile) (geppettobootstrap.ProfileSummaryReport, error) {
	if report != nil {
		for _, summary := range report.Profiles {
			if summary.Registry == registrySlug && summary.Slug == profileSlug {
				return summary, nil
			}
		}
	}
	if profile == nil {
		return geppettobootstrap.ProfileSummaryReport{}, fmt.Errorf("profile %s/%s not found", registrySlug, profileSlug)
	}
	summary := summarizeSettings(profile.InferenceSettings)
	model, apiType := summary.ChatEngine, summary.ChatAPIType
	return geppettobootstrap.ProfileSummaryReport{
		Registry:    registrySlug,
		Slug:        profileSlug,
		DisplayName: strings.TrimSpace(profile.DisplayName),
		Description: strings.TrimSpace(profile.Description),
		Model:       model,
		APIType:     apiType,
		Version:     profile.Metadata.Version,
		Source:      strings.TrimSpace(profile.Metadata.Source),
	}, nil
}
