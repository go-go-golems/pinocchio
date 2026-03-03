package main

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/pinocchio/pkg/ui/runtime"
)

// This is a compile-check POC demonstrating that a third-party module can wire
// Pinocchio’s reusable ChatBuilder without importing any pinocchio/cmd/... packages.
//
// Running this requires valid provider settings/API keys; the goal here is compilation.
func main() {
	ctx := context.Background()

	stepSettings, err := settings.NewStepSettings()
	if err != nil {
		panic(err)
	}

	router, err := events.NewEventRouter()
	if err != nil {
		panic(err)
	}

	ef := factory.NewStandardEngineFactory()

	sess, prog, err := runtime.NewChatBuilder().
		WithEngineFactory(ef).
		WithSettings(stepSettings).
		WithRouter(router).
		WithProgramOptions(tea.WithAltScreen()).
		WithModelOptions(boba_chat.WithTitle("thirdparty-poc")).
		BuildProgram()
	if err != nil {
		panic(err)
	}

	router.AddHandler("ui", "ui", sess.EventHandler())
	if err := router.RunHandlers(ctx); err != nil {
		panic(err)
	}

	_, err = prog.Run()
	if err != nil {
		panic(err)
	}
}
