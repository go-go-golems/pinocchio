package cmds

import (
	"fmt"
	"os"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/middlewares"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/spf13/cobra"
)

func BuildCobraCommandWithGeppettoMiddlewares(
	cmd cmds.Command,
	options ...cli.CobraParserOption,
) (*cobra.Command, error) {
	options_ := append([]cli.CobraParserOption{
		cli.WithCobraMiddlewaresFunc(GetCobraCommandGeppettoMiddlewares),
		cli.WithCobraShortHelpLayers(layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
	}, options...)

	return cli.BuildCobraCommandFromCommand(cmd, options_...)
}

func GetCobraCommandGeppettoMiddlewares(
	parsedCommandLayers *layers.ParsedLayers,
	cmd *cobra.Command,
	args []string,
) ([]middlewares.Middleware, error) {
	commandSettings := &cli.CommandSettings{}
	err := parsedCommandLayers.InitializeStruct(cli.CommandSettingsSlug, commandSettings)
	if err != nil {
		return nil, err
	}

	profileSettings := &cli.ProfileSettings{}
	err = parsedCommandLayers.InitializeStruct(cli.ProfileSettingsSlug, profileSettings)
	if err != nil {
		return nil, err
	}

	// if we want profile support here, we would have to check for a --profile and --profile-file flag,
	// then load the file (or the default file), check for the profile values, then apply them before load-parameters-from-file

	middlewares_ := []middlewares.Middleware{
		middlewares.ParseFromCobraCommand(cmd,
			parameters.WithParseStepSource("cobra"),
		),
		middlewares.GatherArguments(args,
			parameters.WithParseStepSource("arguments"),
		),
	}

	if commandSettings.LoadParametersFromFile != "" {
		middlewares_ = append(middlewares_,
			middlewares.LoadParametersFromFile(commandSettings.LoadParametersFromFile))
	}

	xdgConfigPath, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	// TODO(manuel, 2024-03-20) I wonder if we should just use a custom layer for the profiles, as we want to load
	// the profile from the environment as well. So the sequence would be defaults -> viper -> command line
	defaultProfileFile := fmt.Sprintf("%s/pinocchio/profiles.yaml", xdgConfigPath)
	if profileSettings.ProfileFile == "" {
		profileSettings.ProfileFile = defaultProfileFile
	}
	if profileSettings.Profile == "" {
		profileSettings.Profile = "default"
	}
	middlewares_ = append(middlewares_,
		middlewares.GatherFlagsFromProfiles(
			defaultProfileFile,
			profileSettings.ProfileFile,
			profileSettings.Profile,
			parameters.WithParseStepSource("profiles"),
			parameters.WithParseStepMetadata(map[string]interface{}{
				"profileFile": profileSettings.ProfileFile,
				"profile":     profileSettings.Profile,
			}),
		),
	)

	middlewares_ = append(middlewares_,
		middlewares.WrapWithWhitelistedLayers(
			[]string{
				settings.AiChatSlug,
				settings.AiClientSlug,
				openai.OpenAiChatSlug,
				claude.ClaudeChatSlug,
				cmdlayers.GeppettoHelpersSlug,
			},
			middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
		),
		middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
	)

	return middlewares_, nil
}
