package openai

import (
	"context"
	_ "embed"

	settings2 "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	ai_types "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	geppetto_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/mb0/glob"
	"github.com/pkg/errors"
	openai2 "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
)

var OpenaiCmd = &cobra.Command{
	Use:   "openai",
	Short: "OpenAI commands",
}

type ListEnginesCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &ListEnginesCommand{}

func NewListEngineCommand() (*ListEnginesCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedSection()
	if err != nil {
		return nil, err
	}
	openaiParameterLayer, err := openai.NewValueSection()
	if err != nil {
		return nil, err
	}

	return &ListEnginesCommand{
		CommandDescription: cmds.NewCommandDescription(
			"list-engines",
			cmds.WithShort("list engines"),
			cmds.WithFlags(
				fields.New(
					"id",
					fields.TypeString,
					fields.WithHelp("glob to match engine id"),
				),
				fields.New(
					"owner",
					fields.TypeString,
					fields.WithHelp("glob to match engine owner"),
				),

				fields.New(
					"only-ready",
					fields.TypeBool,
					fields.WithHelp("glob to match engine ready"),
					fields.WithDefault(false),
				),
			),
			cmds.WithSections(
				glazedParameterLayer,
				openaiParameterLayer,
			),
		),
	}, nil
}

type ListEnginesSettings struct {
	ID        string `glazed:"id"`
	Owner     string `glazed:"owner"`
	OnlyReady bool   `glazed:"only-ready"`
}

func (c *ListEnginesCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	s := &ListEnginesSettings{}
	err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s)
	if err != nil {
		return err
	}

	openaiSettings := &openai.Settings{}
	err = parsedLayers.DecodeSectionInto(openai.OpenAiChatSlug, openaiSettings)
	if err != nil {
		return err
	}

	apiSettings := &settings2.APISettings{}
	err = parsedLayers.DecodeSectionInto(openai.OpenAiChatSlug, apiSettings)
	if err != nil {
		return err
	}

	openaiKey, ok := apiSettings.APIKeys[string(ai_types.ApiTypeOpenAI)+"-api-key"]
	if !ok {
		return errors.New("no openai api key")
	}

	client := openai2.NewClient(openaiKey)

	engines, err := client.ListEngines(ctx)
	if err != nil {
		return err
	}

	for _, engine := range engines.Engines {
		if s.ID != "" {
			// check if idGlob  matches id
			matching, err := glob.Match(s.ID, engine.ID)
			if err != nil {
				return err
			}

			if !matching {
				continue
			}
		}

		if s.Owner != "" {
			// check if ownerGlob matches owner
			matching, err := glob.Match(s.Owner, engine.Owner)
			if err != nil {
				return err
			}

			if !matching {
				continue
			}
		}

		if s.OnlyReady {
			if !engine.Ready {
				continue
			}
		}

		row := types.NewRow(
			types.MRP("id", engine.ID),
			types.MRP("owner", engine.Owner),
			types.MRP("ready", engine.Ready),
			types.MRP("object", engine.Object),
		)
		err = gp.AddRow(ctx, row)
		if err != nil {
			return err
		}
	}

	return nil
}

type EngineInfoCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &EngineInfoCommand{}

func NewEngineInfoCommand() (*EngineInfoCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedSection()
	if err != nil {
		return nil, err
	}
	openaiParameterLayer, err := openai.NewValueSection()
	if err != nil {
		return nil, err
	}

	return &EngineInfoCommand{
		CommandDescription: cmds.NewCommandDescription(
			"engine-info",
			cmds.WithShort("get engine info"),
			cmds.WithArguments(
				fields.New(
					"engine",
					fields.TypeString,
					fields.WithHelp("engine id"),
				),
			),
			cmds.WithSections(
				glazedParameterLayer,
				openaiParameterLayer,
			),
		),
	}, nil
}

type EngineInfoSettings struct {
	Engine string `glazed:"engine"`
}

func (c *EngineInfoCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	s := &EngineInfoSettings{}
	err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s)
	if err != nil {
		return err
	}

	openaiSettings := &openai.Settings{}
	err = parsedLayers.DecodeSectionInto(openai.OpenAiChatSlug, openaiSettings)
	cobra.CheckErr(err)

	apiSettings := &settings2.APISettings{}
	err = parsedLayers.DecodeSectionInto(openai.OpenAiChatSlug, apiSettings)
	if err != nil {
		return err
	}

	openaiKey, ok := apiSettings.APIKeys[string(ai_types.ApiTypeOpenAI)+"-api-key"]
	if !ok {
		return errors.New("no openai api key")
	}

	client := openai2.NewClient(openaiKey)

	resp, err := client.GetEngine(ctx, s.Engine)
	cobra.CheckErr(err)

	row := types.NewRow(
		types.MRP("id", resp.ID),
		types.MRP("owner", resp.Owner),
		types.MRP("ready", resp.Ready),
		types.MRP("object", resp.Object),
	)
	err = gp.AddRow(ctx, row)
	cobra.CheckErr(err)

	return nil
}

func init() {
	listEnginesCommand, err := NewListEngineCommand()
	cobra.CheckErr(err)
	listEnginesCobraCommand, err := geppetto_cmds.BuildCobraCommandWithGeppettoMiddlewares(listEnginesCommand)
	cobra.CheckErr(err)
	OpenaiCmd.AddCommand(listEnginesCobraCommand)

	engineInfoCommand, err := NewEngineInfoCommand()
	cobra.CheckErr(err)
	cobraEngineInfoCommand, err := geppetto_cmds.BuildCobraCommandWithGeppettoMiddlewares(engineInfoCommand)
	cobra.CheckErr(err)
	OpenaiCmd.AddCommand(cobraEngineInfoCommand)

	transcribeCommand, err := NewTranscribeCommand()
	cobra.CheckErr(err)
	cobraTranscribeCommand, err := geppetto_cmds.BuildCobraCommandWithGeppettoMiddlewares(transcribeCommand)
	cobra.CheckErr(err)
	OpenaiCmd.AddCommand(cobraTranscribeCommand)
}
