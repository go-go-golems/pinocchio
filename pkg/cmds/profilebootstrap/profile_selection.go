package profilebootstrap

import (
	"context"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	glazedconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/go-go-golems/pinocchio/pkg/configdoc"
	"github.com/pkg/errors"
)

const ProfileSettingsSectionSlug = bootstrap.ProfileSettingsSectionSlug

type ProfileSettings = bootstrap.ProfileSettings
type ResolvedCLIConfigFiles = bootstrap.ResolvedCLIConfigFiles
type CLISelectionInput = bootstrap.CLISelectionInput

type ResolvedCLIProfileRuntime struct {
	ProfileSettings      ProfileSettings
	ConfigFiles          *ResolvedCLIConfigFiles
	Documents            *configdoc.ResolvedDocuments
	Effective            *configdoc.Document
	ProfileRegistryChain *bootstrap.ResolvedProfileRegistryChain
	Close                func()
}

type resolvedConfigRuntime struct {
	ConfigFiles     *ResolvedCLIConfigFiles
	Documents       *configdoc.ResolvedDocuments
	Effective       *configdoc.Document
	ProfileSettings ProfileSettings
}

func pinocchioBootstrapConfig() bootstrap.AppBootstrapConfig {
	return bootstrap.AppBootstrapConfig{
		AppName:          "pinocchio",
		EnvPrefix:        "PINOCCHIO",
		ConfigFileMapper: configFileMapper,
		NewProfileSection: func() (schema.Section, error) {
			return geppettosections.NewProfileSettingsSection()
		},
		BuildBaseSections: func() ([]schema.Section, error) {
			return geppettosections.CreateGeppettoSections()
		},
		ConfigPlanBuilder: pinocchioConfigPlanBuilder,
	}
}

func BootstrapConfig() bootstrap.AppBootstrapConfig {
	return pinocchioBootstrapConfig()
}

func NewProfileSettingsSection() (schema.Section, error) {
	return bootstrap.NewProfileSettingsSection(pinocchioBootstrapConfig())
}

func NewCLISelectionValues(input CLISelectionInput) (*values.Values, error) {
	return bootstrap.NewCLISelectionValues(pinocchioBootstrapConfig(), input)
}

func ResolveCLIConfigFilesResolved(parsed *values.Values) (*ResolvedCLIConfigFiles, error) {
	return bootstrap.ResolveCLIConfigFilesResolved(pinocchioBootstrapConfig(), parsed)
}

func ResolveCLIProfileRuntime(ctx context.Context, parsed *values.Values) (*ResolvedCLIProfileRuntime, error) {
	resolvedConfig, err := resolveConfigRuntime(parsed)
	if err != nil {
		return nil, err
	}

	selection := bootstrap.PrepareProfileSettingsForRuntime(pinocchioBootstrapConfig(), resolvedConfig.ProfileSettings)
	hasInlineProfiles := resolvedConfig.Effective != nil && len(resolvedConfig.Effective.Profiles) > 0
	if selection.Profile != "" && len(selection.ProfileRegistries) == 0 && !hasInlineProfiles {
		return nil, &gepprofiles.ValidationError{
			Field:  "profile-settings.profile",
			Reason: "requires either inline profiles or configured profile registries",
		}
	}

	var imported *bootstrap.ResolvedProfileRegistryChain
	if len(selection.ProfileRegistries) > 0 {
		imported, err = bootstrap.ResolveProfileRegistryChain(ctx, ProfileSettings{
			ProfileRegistries: selection.ProfileRegistries,
		})
		if err != nil {
			return nil, err
		}
	}

	var inlineRegistry gepprofiles.Registry
	var inlineDefaultRegistry gepprofiles.RegistrySlug
	if hasInlineProfiles {
		storeRegistry, err := configdoc.NewInlineStoreRegistry(resolvedConfig.Effective, gepprofiles.MustRegistrySlug(configdoc.DefaultInlineRegistrySlug))
		if err != nil {
			if imported != nil && imported.Close != nil {
				imported.Close()
			}
			return nil, err
		}
		inlineRegistry = storeRegistry
		inlineDefaultRegistry = gepprofiles.MustRegistrySlug(configdoc.DefaultInlineRegistrySlug)
	}

	composed := configdoc.ComposeRegistry(registryFromResolved(imported), registryAsStore(inlineRegistry))
	defaultRegistrySlug := inlineDefaultRegistry
	if defaultRegistrySlug.IsZero() && imported != nil {
		defaultRegistrySlug = imported.DefaultRegistrySlug
	}

	defaultResolve := gepprofiles.ResolveInput{}
	if !defaultRegistrySlug.IsZero() {
		defaultResolve.RegistrySlug = defaultRegistrySlug
	}
	if selection.Profile != "" {
		profileSlug, err := gepprofiles.ParseEngineProfileSlug(selection.Profile)
		if err != nil {
			if imported != nil && imported.Close != nil {
				imported.Close()
			}
			return nil, err
		}
		defaultResolve.EngineProfileSlug = profileSlug
	}

	registryChain := &bootstrap.ResolvedProfileRegistryChain{
		Registry:              composed,
		Reader:                composed,
		DefaultRegistrySlug:   defaultRegistrySlug,
		DefaultProfileResolve: defaultResolve,
	}
	if imported != nil && imported.Close != nil {
		registryChain.Close = func() {
			imported.Close()
		}
	}

	return &ResolvedCLIProfileRuntime{
		ProfileSettings:      selection,
		ConfigFiles:          resolvedConfig.ConfigFiles,
		Documents:            resolvedConfig.Documents,
		Effective:            resolvedConfig.Effective,
		ProfileRegistryChain: registryChain,
		Close:                registryChain.Close,
	}, nil
}

func (r *ResolvedCLIProfileRuntime) Registry() gepprofiles.Registry {
	if r == nil || r.ProfileRegistryChain == nil {
		return nil
	}
	return r.ProfileRegistryChain.Registry
}

func (r *ResolvedCLIProfileRuntime) Reader() gepprofiles.RegistryReader {
	if r == nil || r.ProfileRegistryChain == nil {
		return nil
	}
	return r.ProfileRegistryChain.Reader
}

func resolveConfigRuntime(parsed *values.Values) (*resolvedConfigRuntime, error) {
	configFiles, err := ResolveCLIConfigFilesResolved(parsed)
	if err != nil {
		return nil, err
	}

	documents, err := configdoc.LoadResolvedDocuments(configFiles.Files)
	if err != nil {
		return nil, err
	}
	effective := documents.Effective
	selection := ProfileSettings{}
	if effective != nil {
		selection.Profile = strings.TrimSpace(effective.Profile.Active)
		selection.ProfileRegistries = append([]string(nil), effective.Profile.Registries...)
	}

	explicitSelection := bootstrap.ResolveProfileSettings(parsed)
	if explicitSelection.Profile != "" {
		selection.Profile = explicitSelection.Profile
	}
	if len(explicitSelection.ProfileRegistries) > 0 {
		selection.ProfileRegistries = append([]string(nil), explicitSelection.ProfileRegistries...)
	}
	selection.Profile = strings.TrimSpace(selection.Profile)
	selection.ProfileRegistries = normalizeProfileRegistryEntries(selection.ProfileRegistries)

	return &resolvedConfigRuntime{
		ConfigFiles:     configFiles,
		Documents:       documents,
		Effective:       effective,
		ProfileSettings: selection,
	}, nil
}

func registryFromResolved(resolved *bootstrap.ResolvedProfileRegistryChain) gepprofiles.Registry {
	if resolved == nil {
		return nil
	}
	return resolved.Registry
}

func registryAsStore(reg gepprofiles.Registry) *gepprofiles.StoreRegistry {
	if reg == nil {
		return nil
	}
	store, _ := reg.(*gepprofiles.StoreRegistry)
	return store
}

func normalizeProfileRegistryEntries(entries []string) []string {
	ret := make([]string, 0, len(entries))
	for _, entry := range entries {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			ret = append(ret, trimmed)
		}
	}
	return ret
}

func MapPinocchioConfigFile(rawConfig interface{}) (map[string]map[string]interface{}, error) {
	return configFileMapper(rawConfig)
}

func pinocchioConfigPlanBuilder(parsed *values.Values) (*glazedconfig.Plan, error) {
	explicit := ""
	if parsed != nil {
		commandSettings := &cli.CommandSettings{}
		if err := parsed.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings); err == nil {
			explicit = strings.TrimSpace(commandSettings.ConfigFile)
		}
	}

	return glazedconfig.NewPlan(
		glazedconfig.WithLayerOrder(
			glazedconfig.LayerSystem,
			glazedconfig.LayerUser,
			glazedconfig.LayerRepo,
			glazedconfig.LayerCWD,
			glazedconfig.LayerExplicit,
		),
		glazedconfig.WithDedupePaths(),
	).Add(
		glazedconfig.SystemAppConfig("pinocchio").Named("system-app-config").Kind("app-config"),
		glazedconfig.HomeAppConfig("pinocchio").Named("home-app-config").Kind("app-config"),
		glazedconfig.XDGAppConfig("pinocchio").Named("xdg-app-config").Kind("app-config"),
		glazedconfig.GitRootFile(configdoc.LocalOverrideFileName).Named("git-root-local-profile").Kind("profile-overlay"),
		glazedconfig.GitRootFile(configdoc.LocalProjectOverrideFileName).Named("git-root-local-profile-override").Kind("profile-overlay"),
		glazedconfig.WorkingDirFile(configdoc.LocalOverrideFileName).Named("cwd-local-profile").Kind("profile-overlay"),
		glazedconfig.WorkingDirFile(configdoc.LocalProjectOverrideFileName).Named("cwd-local-profile-override").Kind("profile-overlay"),
		glazedconfig.ExplicitFile(explicit).Named("explicit-config-file").Kind("explicit-file"),
	), nil
}

func configFileMapper(rawConfig interface{}) (map[string]map[string]interface{}, error) {
	configMap, ok := rawConfig.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("expected map[string]interface{}, got %T", rawConfig)
	}

	result := make(map[string]map[string]interface{})

	for section, value := range configMap {
		if sectionMap, ok := value.(map[string]interface{}); ok {
			result[section] = sectionMap
		}
	}

	if profileBlock, ok := configMap["profile"].(map[string]interface{}); ok {
		mapped := map[string]interface{}{}
		if active, ok := profileBlock["active"].(string); ok && strings.TrimSpace(active) != "" {
			mapped["profile"] = strings.TrimSpace(active)
		}
		switch registries := profileBlock["registries"].(type) {
		case []interface{}:
			out := []string{}
			for _, entry := range registries {
				if s, ok := entry.(string); ok && strings.TrimSpace(s) != "" {
					out = append(out, strings.TrimSpace(s))
				}
			}
			if len(out) > 0 {
				mapped["profile-registries"] = out
			}
		case []string:
			out := []string{}
			for _, entry := range registries {
				if trimmed := strings.TrimSpace(entry); trimmed != "" {
					out = append(out, trimmed)
				}
			}
			if len(out) > 0 {
				mapped["profile-registries"] = out
			}
		}
		if len(mapped) > 0 {
			result[ProfileSettingsSectionSlug] = mapped
		}
	}
	return result, nil
}

var _ sources.ConfigFileMapper = configFileMapper
