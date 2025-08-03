package run

import (
	"io"
	"os"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

type RunMode int

const (
	RunModeBlocking RunMode = iota
	RunModeInteractive
	RunModeChat
)

// UISettings contains all settings related to terminal UI and output formatting
type UISettings struct {
	Interactive      bool
	ForceInteractive bool
	NonInteractive   bool
	StartInChat      bool
	PrintPrompt      bool
	Output           string
	WithMetadata     bool
	FullOutput       bool
	UseStepBackend   bool
}

// RunContext encapsulates all the settings and state needed for a single command run
type RunContext struct {
	// Core components (ConversationManager is required)
	ConversationManager conversation.Manager

	StepSettings *settings.StepSettings

	EngineFactory inference.EngineFactory
	Router        *events.EventRouter

	// Optional UI/Terminal specific components
	UISettings *UISettings
	Writer     io.Writer

	// Run configuration
	RunMode RunMode
}

type RunOption func(*RunContext) error

// Core options

func WithStepSettings(settings *settings.StepSettings) RunOption {
	return func(rc *RunContext) error {
		rc.StepSettings = settings
		if rc.EngineFactory == nil {
			rc.EngineFactory = inference.NewStandardEngineFactory()
		}
		return nil
	}
}

func WithEngineFactory(factory inference.EngineFactory) RunOption {
	return func(rc *RunContext) error {
		rc.EngineFactory = factory
		return nil
	}
}

func WithRouter(router *events.EventRouter) RunOption {
	return func(rc *RunContext) error {
		rc.Router = router
		return nil
	}
}

func WithConversationManager(manager conversation.Manager) RunOption {
	return func(rc *RunContext) error {
		rc.ConversationManager = manager
		return nil
	}
}

// Mode options

func WithRunMode(mode RunMode) RunOption {
	return func(rc *RunContext) error {
		rc.RunMode = mode
		return nil
	}
}

// UI options

func WithUISettings(settings *UISettings) RunOption {
	return func(rc *RunContext) error {
		rc.UISettings = settings
		return nil
	}
}

func WithWriter(w io.Writer) RunOption {
	return func(rc *RunContext) error {
		rc.Writer = w
		return nil
	}
}

// NewRunContext creates a new RunContext with default values and a required manager
func NewRunContext(manager conversation.Manager) *RunContext {
	return &RunContext{
		ConversationManager: manager,
		RunMode:             RunModeBlocking,
		Writer:              os.Stdout,
	}
}
