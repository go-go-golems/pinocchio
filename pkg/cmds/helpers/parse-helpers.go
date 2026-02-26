package helpers

import (
	"os"
	"path/filepath"
	"strings"

	embeddings_config "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	geppetto_sections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/gemini"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	appconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
)

type GeppettoLayersHelper struct {
	Profile           string
	ProfileRegistries string
	UseViper          bool
}

type GeppettoLayersHelperOption func(*GeppettoLayersHelper)

func WithProfileRegistries(profileRegistries string) GeppettoLayersHelperOption {
	return func(h *GeppettoLayersHelper) {
		h.ProfileRegistries = profileRegistries
	}
}

func WithProfile(profile string) GeppettoLayersHelperOption {
	return func(h *GeppettoLayersHelper) {
		h.Profile = profile
	}
}

func WithUseViper(useViper bool) GeppettoLayersHelperOption {
	return func(h *GeppettoLayersHelper) {
		h.UseViper = useViper
	}
}

// ParseGeppettoLayers parses the Geppetto layers from the command and returns them, this is a way to parse
// profiles and config file without using the GetGeppettoMiddlewares function which also parses from cobra.
func ParseGeppettoLayers(c *cmds.PinocchioCommand, options ...GeppettoLayersHelperOption) (*values.Values, error) {
	defaultProfileRegistries := defaultPinocchioProfileRegistriesIfPresent()
	profile := strings.TrimSpace(os.Getenv("PINOCCHIO_PROFILE"))
	if profile == "" {
		profile = "default"
	}
	profileRegistries := strings.TrimSpace(os.Getenv("PINOCCHIO_PROFILE_REGISTRIES"))
	if profileRegistries == "" {
		profileRegistries = defaultProfileRegistries
	}
	helper := &GeppettoLayersHelper{
		Profile:           profile,
		ProfileRegistries: profileRegistries,
		UseViper:          true,
	}
	for _, option := range options {
		option(helper)
	}
	middlewares_ := []sources.Middleware{}
	if helper.Profile != "" {
		profileRegistrySources, err := gepprofiles.ParseProfileRegistrySourceEntries(helper.ProfileRegistries)
		if err != nil {
			return nil, err
		}
		if len(profileRegistrySources) == 0 {
			return nil, &gepprofiles.ValidationError{
				Field:  "profile-settings.profile-registries",
				Reason: "must be configured (hard cutover: no profile-file fallback)",
			}
		}
		middlewares_ = append(middlewares_,
			geppetto_sections.GatherFlagsFromProfileRegistry(
				profileRegistrySources,
				helper.Profile,
				fields.WithSource("profiles"),
				fields.WithMetadata(map[string]interface{}{
					"profileRegistries": profileRegistrySources,
					"profile":           helper.Profile,
					"mode":              "profile-registry-stack",
				}),
			),
		)
	}

	if helper.UseViper {
		// Discover config file using ResolveAppConfigPath
		configMiddlewares := []sources.Middleware{}
		configPath, err := appconfig.ResolveAppConfigPath("pinocchio", "")
		if err == nil && configPath != "" {
			configMiddlewares = append(configMiddlewares,
				sources.FromFile(configPath,
					sources.WithParseOptions(fields.WithSource("config")),
				),
			)
		}
		configMiddlewares = append(configMiddlewares,
			sources.FromEnv("PINOCCHIO",
				fields.WithSource("env"),
			),
		)

		middlewares_ = append(middlewares_,
			sources.WrapWithWhitelistedSections(
				[]string{
					settings.AiChatSlug,
					settings.AiClientSlug,
					settings.AiInferenceSlug,
					openai.OpenAiChatSlug,
					claude.ClaudeChatSlug,
					gemini.GeminiChatSlug,
					embeddings_config.EmbeddingsSlug,
					cmdlayers.GeppettoHelpersSlug,
				},
				sources.Chain(configMiddlewares...),
			),
			sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
		)
	}

	geppettoParsedValues := values.New()
	if err := sources.Execute(c.Description().Schema, geppettoParsedValues, middlewares_...); err != nil {
		return nil, err
	}

	return geppettoParsedValues, nil
}

func defaultPinocchioProfileRegistriesIfPresent() string {
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		return ""
	}
	path := filepath.Join(configDir, "pinocchio", "profiles.yaml")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}
