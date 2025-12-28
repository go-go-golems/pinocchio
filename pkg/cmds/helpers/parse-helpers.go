package helpers

import (
	"fmt"
	"os"

	embeddings_config "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/gemini"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/middlewares"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	appconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
)

type GeppettoLayersHelper struct {
	ProfileFile string
	Profile     string
	UseViper    bool
}

type GeppettoLayersHelperOption func(*GeppettoLayersHelper)

func WithProfileFile(profileFile string) GeppettoLayersHelperOption {
	return func(h *GeppettoLayersHelper) {
		h.ProfileFile = profileFile
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
func ParseGeppettoLayers(c *cmds.PinocchioCommand, options ...GeppettoLayersHelperOption) (*layers.ParsedLayers, error) {
	xdgConfigPath, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	defaultProfileFile := fmt.Sprintf("%s/pinocchio/profiles.yaml", xdgConfigPath)
	helper := &GeppettoLayersHelper{
		ProfileFile: defaultProfileFile,
		Profile:     "",
		UseViper:    true,
	}
	for _, option := range options {
		option(helper)
	}
	middlewares_ := []middlewares.Middleware{}
	if helper.Profile != "" {
		middlewares_ = append(middlewares_,
			middlewares.GatherFlagsFromProfiles(
				helper.ProfileFile,
				helper.ProfileFile,
				helper.Profile,
				"default",
				parameters.WithParseStepSource("profiles"),
				parameters.WithParseStepMetadata(map[string]interface{}{
					"profileFile": helper.ProfileFile,
					"profile":     helper.Profile,
				}),
			),
		)
	}

	if helper.UseViper {
		// Discover config file using ResolveAppConfigPath
		configMiddlewares := []middlewares.Middleware{}
		configPath, err := appconfig.ResolveAppConfigPath("pinocchio", "")
		if err == nil && configPath != "" {
			configMiddlewares = append(configMiddlewares,
				middlewares.LoadParametersFromFile(configPath,
					middlewares.WithParseOptions(parameters.WithParseStepSource("config")),
				),
			)
		}
		configMiddlewares = append(configMiddlewares,
			middlewares.UpdateFromEnv("PINOCCHIO",
				parameters.WithParseStepSource("env"),
			),
		)

		middlewares_ = append(middlewares_,
			middlewares.WrapWithWhitelistedLayers(
				[]string{
					settings.AiChatSlug,
					settings.AiClientSlug,
					openai.OpenAiChatSlug,
					claude.ClaudeChatSlug,
					gemini.GeminiChatSlug,
					embeddings_config.EmbeddingsSlug,
					cmdlayers.GeppettoHelpersSlug,
				},
				middlewares.Chain(configMiddlewares...),
			),
			middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
		)
	}

	geppettoParsedLayers := layers.NewParsedLayers()
	err = middlewares.ExecuteMiddlewares(c.Description().Layers, geppettoParsedLayers, middlewares_...)
	if err != nil {
		return nil, err
	}

	return geppettoParsedLayers, nil
}
