package tuidemo

import (
	"context"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/tools"
	toolloopbackend "github.com/go-go-golems/pinocchio/pkg/ui/backends/toolloop"
	agentforwarder "github.com/go-go-golems/pinocchio/pkg/ui/forwarders/agent"
	"golang.org/x/sync/errgroup"
)

type RunSpec struct {
	Title            string
	Engine           engine.Engine
	Registry         *tools.InMemoryToolRegistry
	SystemPrompt     string
	TimelineRegister func(*timeline.Registry)
	StatusBarView    func() string
}

func RunToolLoopDemo(ctx context.Context, spec RunSpec) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	goPubSub := gochannel.NewGoChannel(gochannel.Config{
		OutputChannelBuffer:            256,
		BlockPublishUntilSubscriberAck: false,
	}, watermill.NopLogger{})
	router, err := events.NewEventRouter(
		events.WithPublisher(goPubSub),
		events.WithSubscriber(goPubSub),
	)
	if err != nil {
		return err
	}
	defer func() { _ = router.Close() }()

	sink := middleware.NewWatermillSink(router.Publisher, "chat")
	middlewares := []middleware.Middleware{
		middleware.NewSystemPromptMiddleware(spec.SystemPrompt),
		middleware.NewToolResultReorderMiddleware(),
	}
	backend := toolloopbackend.NewToolLoopBackend(spec.Engine, middlewares, spec.Registry, sink, nil)

	model := chat.InitialModel(backend,
		chat.WithTitle(spec.Title),
		chat.WithTimelineRegister(spec.TimelineRegister),
		chat.WithStatusBarView(spec.StatusBarView),
	)

	program := tea.NewProgram(model, tea.WithAltScreen())
	router.AddHandler("ui-forward", "chat", agentforwarder.MakeUIForwarder(program))

	eg, groupCtx := errgroup.WithContext(ctx)
	eg.Go(func() error { return router.Run(groupCtx) })
	eg.Go(func() error {
		_, err := program.Run()
		cancel()
		return err
	})
	return eg.Wait()
}
