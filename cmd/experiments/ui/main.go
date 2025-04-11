package main

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat/steps"
	"github.com/go-go-golems/pinocchio/pkg/ui"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

func main() {
	log.Logger = log.Logger.Level(zerolog.InfoLevel)
	manager := conversation.NewManager(
		conversation.WithMessages(
			conversation.NewChatMessage(conversation.RoleSystem, "hahahahaha"),
		))

	step := steps.NewEchoStep()

	router, err := events.NewEventRouter()
	if err != nil {
		panic(err)
	}

	err = step.AddPublishedTopic(router.Publisher, "ui")
	if err != nil {
		panic(err)
	}

	eg := errgroup.Group{}
	ctx, cancel := context.WithCancel(context.Background())

	f := func() {
		cancel()
		defer func(Router *events.EventRouter) {
			_ = Router.Close()
		}(router)
	}

	eg.Go(func() error {
		defer f()
		ret := router.Run(ctx)
		if ret != nil {
			return ret
		}
		return nil
	})

	eg.Go(func() error {
		defer f()

		// Wait for router to be ready

		options := []tea.ProgramOption{
			tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
		}
		options = append(options, tea.WithAltScreen())

		backend := ui.NewStepBackend(step)
		p := tea.NewProgram(
			boba_chat.InitialModel(manager, backend,
				boba_chat.WithTitle("ui"),
				boba_chat.WithAutoStartBackend(true),
			),
			options...,
		)

		router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))

		err = router.RunHandlers(ctx)
		if err != nil {
			return err
		}

		if _, err := p.Run(); err != nil {
			panic(err)
		}

		return nil
	})

	err = eg.Wait()
	if err != nil {
		panic(err)
	}
}
