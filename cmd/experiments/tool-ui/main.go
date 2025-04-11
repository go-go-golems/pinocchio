package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/openai"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	glazed_cmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/go-go-golems/pinocchio/pkg/ui"
	"github.com/invopop/jsonschema"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type ToolUiRunner struct {
	stepSettings *settings.StepSettings
	manager      conversation.Manager
	reflector    *jsonschema.Reflector
	chatToolStep *openai.ChatExecuteToolStep
	eventRouter  *events.EventRouter
}

type ToolUiCommand struct {
	*glazed_cmds.CommandDescription
}

var _ glazed_cmds.BareCommand = (*ToolUiCommand)(nil)

func NewToolUiCommand() (*ToolUiCommand, error) {
	stepSettings, err := settings.NewStepSettings()
	if err != nil {
		return nil, err
	}
	geppettoLayers, err := cmds.CreateGeppettoLayers(stepSettings, cmds.WithHelpersLayer())
	if err != nil {
		return nil, err
	}

	return &ToolUiCommand{
		CommandDescription: glazed_cmds.NewCommandDescription(
			"tool-ui",
			glazed_cmds.WithShort("Tool UI"),
			glazed_cmds.WithFlags(
				parameters.NewParameterDefinition(
					"ui",
					parameters.ParameterTypeBool,
					parameters.WithDefault(false),
					parameters.WithHelp("start in UI mode")),
				parameters.NewParameterDefinition(
					"print-raw-events",
					parameters.ParameterTypeBool,
					parameters.WithDefault(false),
					parameters.WithHelp("print raw events")),
				parameters.NewParameterDefinition(
					"verbose",
					parameters.ParameterTypeBool,
					parameters.WithDefault(false),
					parameters.WithHelp("verbose")),
			),
			glazed_cmds.WithLayersList(geppettoLayers...),
		),
	}, nil
}

type ToolUiSettings struct {
	UI             bool `glazed.parameter:"ui"`
	PrintRawEvents bool `glazed.parameter:"print-raw-events"`
	Verbose        bool `glazed.parameter:"verbose"`
}

func (t *ToolUiCommand) Run(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
) error {
	runner := &ToolUiRunner{}
	settings := &ToolUiSettings{}

	err := parsedLayers.InitializeStruct(layers.DefaultSlug, settings)
	if err != nil {
		return err
	}

	if settings.UI {
		return runner.runWithUi(ctx, parsedLayers)
	}

	err = runner.Init(parsedLayers)
	if err != nil {
		return err
	}

	defer func() {
		err := runner.eventRouter.Close()
		if err != nil {
			log.Error().Err(err).Msg("Failed to close eventRouter")
		}
	}()

	if settings.PrintRawEvents {
		runner.eventRouter.AddHandler("raw-events-stdout", "ui", runner.eventRouter.DumpRawEvents)
	} else {
		runner.eventRouter.AddHandler("ui-stdout",
			"ui",
			events.StepPrinterFunc("UI", os.Stdout),
		)
	}

	ctx, cancel := context.WithCancel(ctx)

	eg := errgroup.Group{}
	eg.Go(func() error {
		defer cancel()

		result, err := runner.chatToolStep.Start(ctx, runner.manager.GetConversation())
		if err != nil {
			return err
		}
		res := <-result.GetChannel()
		fmt.Printf("\n\nchatToolStep.Start returned %v\n", res.ValueOr(nil))
		return nil
	})

	eg.Go(func() error {
		ret := runner.eventRouter.Run(ctx)
		fmt.Printf("eventRouter.Run returned %v\n", ret)
		return nil
	})

	err = eg.Wait()
	if err != nil {
		return err
	}

	return nil
}

func (r *ToolUiRunner) Init(parsedLayers *layers.ParsedLayers) error {
	var err error
	r.stepSettings, err = settings.NewStepSettings()
	if err != nil {
		return err
	}
	err = r.stepSettings.UpdateFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	r.stepSettings.Chat.Stream = true

	r.manager = conversation.NewManager(
		conversation.WithMessages(
			conversation.NewChatMessage(
				conversation.RoleUser,
				"Give me the weather in Boston on november 9th 1924, please, including the windspeed for me, an old ass american. Also, the weather in paris today, with temperature.",
			),
		))

	r.eventRouter, err = events.NewEventRouter()
	if err != nil {
		return err
	}

	r.reflector = &jsonschema.Reflector{
		DoNotReference: true,
	}
	err = r.reflector.AddGoComments("github.com/go-go-golems/pinocchio", "./cmd/experiments/tool-ui")
	if err != nil {
		log.Warn().Err(err).Msg("Could not add go comments")
	}

	r.chatToolStep, err = openai.NewChatToolStep(
		r.stepSettings,
		openai.WithReflector(r.reflector),
		openai.WithToolFunctions(map[string]any{
			"getWeather":      getWeather,
			"getWeatherOnDay": getWeatherOnDay,
		}),
	)
	if err != nil {
		return err
	}
	err = r.chatToolStep.AddPublishedTopic(r.eventRouter.Publisher, "ui")
	if err != nil {
		return err
	}

	return nil
}

func (r *ToolUiRunner) runWithUi(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
) error {
	err := r.Init(parsedLayers)
	if err != nil {
		return err
	}

	backend := ui.NewStepBackend(r.chatToolStep)

	// Create bubbletea UI

	options := []tea.ProgramOption{
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	}
	options = append(options, tea.WithAltScreen())

	// maybe test with CLI output first

	p := tea.NewProgram(
		boba_chat.InitialModel(r.manager, backend),
		options...,
	)

	r.eventRouter.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))

	ctx, cancel := context.WithCancel(ctx)

	eg := errgroup.Group{}

	eg.Go(func() error {
		ret := r.eventRouter.Run(ctx)
		fmt.Printf("router.Run returned %v\n", ret)
		return nil
	})

	eg.Go(func() error {
		if _, err := p.Run(); err != nil {
			return err
		}
		defer cancel()
		return nil
	})

	err = eg.Wait()
	if err != nil {
		return err
	}

	return &glazed_cmds.ExitWithoutGlazeError{}
}

func main() {
	toolUiCommand, err := NewToolUiCommand()
	cobra.CheckErr(err)

	toolUICobraCommand, err := cmds.BuildCobraCommandWithGeppettoMiddlewares(toolUiCommand)
	cobra.CheckErr(err)

	err = clay.InitViper("pinocchio", toolUICobraCommand)
	cobra.CheckErr(err)

	err = toolUICobraCommand.Execute()
	cobra.CheckErr(err)
}
