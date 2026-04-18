package helpers

import (
	"strings"

	embeddings_config "github.com/go-go-golems/geppetto/pkg/embeddings/config"
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
	helper := &GeppettoLayersHelper{UseViper: true}
	for _, option := range options {
		option(helper)
	}

	selectionValues, err := profilebootstrap.NewCLISelectionValues(profilebootstrap.CLISelectionInput{
		ConfigFile:        strings.TrimSpace(helper.ConfigFile),
		Profile:           strings.TrimSpace(helper.Profile),
		ProfileRegistries: append([]string(nil), helper.ProfileRegistries...),
	})
	if err != nil {
		return nil, err
	}
	profileSelection, err := profilebootstrap.ResolveCLIProfileSelection(selectionValues)
	if err != nil {
		return nil, err
	}

	middlewares_ := []sources.Middleware{}
	if helper.UseViper {
		configMiddlewares := []sources.Middleware{}
		for _, configPath := range profileSelection.ConfigFiles {
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
