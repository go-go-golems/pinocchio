package run

import (
	"errors"
	"io"
	"os"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps/ai"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
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
}

// RunContext encapsulates all the settings and state needed for a single command run
type RunContext struct {
	// Core components (Manager is required)
	Manager conversation.Manager

	StepSettings *settings.StepSettings

	StepFactory *ai.StandardStepFactory
	Router      *events.EventRouter

	// Optional UI/Terminal specific components
	UISettings *UISettings
	Writer     io.Writer

	// Run configuration
	RunMode RunMode

	// Variables for templating
	Variables map[string]interface{}
}

type RunOption func(*RunContext) error

// Core options

func WithStepSettings(settings *settings.StepSettings) RunOption {
	return func(rc *RunContext) error {
		rc.StepSettings = settings
		if rc.StepFactory == nil {
			rc.StepFactory = &ai.StandardStepFactory{Settings: settings}
		}
		return nil
	}
}

func WithRouter(router *events.EventRouter) RunOption {
	return func(rc *RunContext) error {
		rc.Router = router
		return nil
	}
}

func WithManager(manager conversation.Manager) RunOption {
	return func(rc *RunContext) error {
		rc.Manager = manager
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

// Variables and parsed layers

func WithVariables(variables map[string]interface{}) RunOption {
	return func(rc *RunContext) error {
		rc.Variables = variables
		return nil
	}
}

func WithParsedLayers(parsedLayers *layers.ParsedLayers) RunOption {
	return func(rc *RunContext) error {
		val, present := parsedLayers.Get(layers.DefaultSlug)
		if !present {
			return errors.New("could not get default layer")
		}
		rc.Variables = val.Parameters.ToMap()
		return nil
	}
}

// NewRunContext creates a new RunContext with default values and a required manager
func NewRunContext(manager conversation.Manager) *RunContext {
	return &RunContext{
		Manager:   manager,
		RunMode:   RunModeBlocking,
		Writer:    os.Stdout,
		Variables: make(map[string]interface{}),
	}
}
