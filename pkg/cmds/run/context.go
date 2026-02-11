package run

import (
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"io"
	"os"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
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

type PersistenceSettings struct {
	TimelineDSN string
	TimelineDB  string
	TurnsDSN    string
	TurnsDB     string
}

// RunContext encapsulates all the settings and state needed for a single command run
type RunContext struct {
	StepSettings *settings.StepSettings

	EngineFactory factory.EngineFactory
	Router        *events.EventRouter

	// Template variables used to render prompts/messages prior to model calls
	Variables map[string]interface{}

	// ImagePaths are CLI-provided image paths to attach to the initial user message (if any).
	ImagePaths []string

	// ResultTurn stores the resulting Turn after engine execution when needed by callers
	ResultTurn *turns.Turn

	// Optional UI/Terminal specific components
	UISettings  *UISettings
	Writer      io.Writer
	Persistence PersistenceSettings

	// Run configuration
	RunMode RunMode
}

type RunOption func(*RunContext) error

// Core options

func WithStepSettings(settings *settings.StepSettings) RunOption {
	return func(rc *RunContext) error {
		rc.StepSettings = settings
		if rc.EngineFactory == nil {
			rc.EngineFactory = factory.NewStandardEngineFactory()
		}
		return nil
	}
}

func WithEngineFactory(factory factory.EngineFactory) RunOption {
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

func WithPersistenceSettings(settings PersistenceSettings) RunOption {
	return func(rc *RunContext) error {
		rc.Persistence = settings
		return nil
	}
}

// WithVariables passes a map of template variables used to render
// system prompt, messages and user prompt before sending to the model.
func WithVariables(vars map[string]interface{}) RunOption {
	return func(rc *RunContext) error {
		rc.Variables = vars
		return nil
	}
}

// WithImagePaths passes a list of images that should be attached to the initial user prompt.
func WithImagePaths(imagePaths []string) RunOption {
	return func(rc *RunContext) error {
		rc.ImagePaths = imagePaths
		return nil
	}
}

// NewRunContext creates a new RunContext with default values and a required manager
func NewRunContext() *RunContext {
	return &RunContext{
		RunMode: RunModeBlocking,
		Writer:  os.Stdout,
	}
}
