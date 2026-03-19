package helpers

import (
	"os"
	"strings"

	embeddings_config "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/gemini"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

type GeppettoLayersHelper struct {
	Profile           string
	ProfileRegistries []string
	ConfigFile        string
	UseViper          bool
}

type GeppettoLayersHelperOption func(*GeppettoLayersHelper)

func WithProfileRegistries(profileRegistries []string) GeppettoLayersHelperOption {
	return func(h *GeppettoLayersHelper) {
		h.ProfileRegistries = append([]string(nil), profileRegistries...)
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

func WithConfigFile(configFile string) GeppettoLayersHelperOption {
	return func(h *GeppettoLayersHelper) {
		h.ConfigFile = configFile
	}
}

// ParseGeppettoLayers parses the Geppetto layers from the command and returns them, this is a way to parse
// profiles and config file without using the GetGeppettoMiddlewares function which also parses from cobra.
func ParseGeppettoLayers(c *cmds.PinocchioCommand, options ...GeppettoLayersHelperOption) (*values.Values, error) {
	profile := strings.TrimSpace(os.Getenv("PINOCCHIO_PROFILE"))
	profileRegistries := normalizeRegistryList(strings.Split(strings.TrimSpace(os.Getenv("PINOCCHIO_PROFILE_REGISTRIES")), ","))
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
		helper.ProfileRegistries = normalizeRegistryList(helper.ProfileRegistries)
		if len(helper.ProfileRegistries) == 0 {
			return nil, &gepprofiles.ValidationError{
				Field:  "profile-settings.profile-registries",
				Reason: "must be configured when profile-settings.profile is set",
			}
		}
	}

	if helper.UseViper {
		configMiddlewares := []sources.Middleware{}
		configFiles, err := profilebootstrap.ResolveCLIConfigFiles(nil)
		if err != nil {
			return nil, err
		}
		if explicit := strings.TrimSpace(helper.ConfigFile); explicit != "" {
			explicitFiles, err := profilebootstrap.ResolveCLIConfigFilesForExplicit(explicit)
			if err != nil {
				return nil, err
			}
			configFiles = explicitFiles
		}
		for _, configPath := range configFiles {
			configMiddlewares = append(configMiddlewares,
				sources.FromFile(configPath,
					sources.WithConfigFileMapper(profilebootstrap.MapPinocchioConfigFile),
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

func normalizeRegistryList(entries []string) []string {
	ret := make([]string, 0, len(entries))
	for _, entry := range entries {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			ret = append(ret, trimmed)
		}
	}
	return ret
}
