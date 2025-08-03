package main

import (
	"context"
	"fmt"
	"os"

	bobachat "github.com/go-go-golems/bobatea/pkg/chat"
	geppetto_conversation "github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/inference"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/pinocchio/pkg/chatrunner"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Configure logging
	logFile, err := os.OpenFile("/tmp/ui.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	log.Logger = log.With().CallerWithSkipFrameCount(0).Logger().Output(zerolog.ConsoleWriter{Out: logFile})
	zerolog.SetGlobalLevel(zerolog.DebugLevel) // Use Debug level for more verbose output

	// 1. Create Conversation Manager
	manager := geppetto_conversation.NewManager(
		geppetto_conversation.WithMessages(
			geppetto_conversation.NewChatMessage(geppetto_conversation.RoleSystem, "System Prompt: You are helpful."),
		),
	)

	// 2. Create Step Settings
	stepSettings, err := settings.NewStepSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create step settings")
		os.Exit(1)
	}

	// 3. Create Engine Factory
	engineFactory := inference.NewStandardEngineFactory()

	// 4. Use the ChatBuilder
	log.Info().Msg("Configuring Chat Runner")
	builder := chatrunner.NewChatBuilder().
		WithManager(manager).
		WithEngineFactory(engineFactory).
		WithSettings(stepSettings).
		WithMode(chatrunner.RunModeChat).
		WithUIOptions(bobachat.WithTitle("Engine Chat Runner")). // Customize UI title
		WithContext(context.Background())

	// 5. Run the chat session
	log.Info().Msg("Starting Chat Runner")
	session, err := builder.Build()
	if err != nil {
		log.Error().Err(err).Msg("Failed to build chat runner")
		os.Exit(1)
	}
	err = session.Run()

	// 6. Handle potential errors
	if err != nil {
		log.Error().Err(err).Msg("Chat Runner failed")
		// Use fmt.Fprintf for cleaner exit message without log formatting
		_, _ = fmt.Fprintf(os.Stderr, "Error running chat: %v\n", err)
		os.Exit(1)
	}

	log.Info().Msg("Chat Runner finished successfully")
}
