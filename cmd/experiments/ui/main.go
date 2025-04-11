package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ThreeDotsLabs/watermill/message"
	bobachat "github.com/go-go-golems/bobatea/pkg/chat"
	geppetto_conversation "github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat/steps"
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

	// 2. Define Step Factory
	// This factory creates a new EchoStep instance when needed by the runner.
	// It configures the step with the provided publisher and topic.
	stepFactory := func(publisher message.Publisher, topic string) (chat.Step, error) {
		step := steps.NewEchoStep() // Create the base step
		if publisher != nil && topic != "" {
			// Configure the step to publish events *if* a publisher and topic are given
			err := step.AddPublishedTopic(publisher, topic)
			if err != nil {
				return nil, err
			}
		}
		return step, nil
	}

	// 3. Use the ChatBuilder
	log.Info().Msg("Configuring Chat Runner")
	builder := chatrunner.NewChatBuilder().
		WithManager(manager).
		WithStepFactory(stepFactory).
		WithMode(chatrunner.RunModeChat).
		WithUIOptions(bobachat.WithTitle("Echo Chat Runner")). // Customize UI title
		WithContext(context.Background())

	// 4. Run the chat session
	log.Info().Msg("Starting Chat Runner")
	session, err := builder.Build()
	if err != nil {
		log.Error().Err(err).Msg("Failed to build chat runner")
		os.Exit(1)
	}
	err = session.Run()

	// 5. Handle potential errors
	if err != nil {
		log.Error().Err(err).Msg("Chat Runner failed")
		// Use fmt.Fprintf for cleaner exit message without log formatting
		_, _ = fmt.Fprintf(os.Stderr, "Error running chat: %v\n", err)
		os.Exit(1)
	}

	log.Info().Msg("Chat Runner finished successfully")
}
