package run

import (
	"io"
	"os"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps/ai"
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
	// Core components
	Manager     conversation.Manager
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

// NewRunContext creates a new RunContext with default values
func NewRunContext() *RunContext {
	return &RunContext{
		RunMode:   RunModeBlocking,
		Writer:    os.Stdout,
		Variables: make(map[string]interface{}),
	}
}
