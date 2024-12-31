package cmdcontext

import (
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps/ai"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/pkg/errors"
)

type CommandContext struct {
	Router              *events.EventRouter
	ConversationManager conversation.Manager
	StepFactory         *ai.StandardStepFactory
	Settings            *cmdlayers.HelpersSettings
}

type CommandContextOption func(*CommandContext) error

func WithCommandContextRouter(router *events.EventRouter) CommandContextOption {
	return func(c *CommandContext) error {
		c.Router = router
		return nil
	}
}

func WithCommandContextConversationManager(manager conversation.Manager) CommandContextOption {
	return func(c *CommandContext) error {
		c.ConversationManager = manager
		return nil
	}
}

func WithCommandContextStepFactory(factory *ai.StandardStepFactory) CommandContextOption {
	return func(c *CommandContext) error {
		c.StepFactory = factory
		return nil
	}
}

func WithCommandContextSettings(settings *cmdlayers.HelpersSettings) CommandContextOption {
	return func(c *CommandContext) error {
		c.Settings = settings
		return nil
	}
}

func NewCommandContext(options ...CommandContextOption) (*CommandContext, error) {
	ctx := &CommandContext{}
	for _, opt := range options {
		if err := opt(ctx); err != nil {
			return nil, err
		}
	}
	return ctx, nil
}

func NewCommandContextFromLayers(parsedLayers *layers.ParsedLayers, stepSettings *settings.StepSettings) (*CommandContext, error) {
	settings := &cmdlayers.HelpersSettings{}
	err := parsedLayers.InitializeStruct(cmdlayers.GeppettoHelpersSlug, settings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize settings")
	}

	err = stepSettings.UpdateFromParsedLayers(parsedLayers)
	if err != nil {
		return nil, err
	}

	stepFactory := &ai.StandardStepFactory{
		Settings: stepSettings,
	}

	router, err := events.NewEventRouter()
	if err != nil {
		return nil, err
	}

	conversationManager := conversation.NewManager(
		conversation.WithAutosave(
			settings.Autosave.Enabled,
			settings.Autosave.Template,
			settings.Autosave.Path,
		),
	)

	return NewCommandContext(
		WithCommandContextRouter(router),
		WithCommandContextConversationManager(conversationManager),
		WithCommandContextStepFactory(stepFactory),
		WithCommandContextSettings(settings),
	)
}
