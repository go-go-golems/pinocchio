package main

import (
	"context"
	"io"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"
	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	renderers "github.com/go-go-golems/bobatea/pkg/timeline/renderers"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/tools"
	geppettolayers "github.com/go-go-golems/geppetto/pkg/layers"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	backendpkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/backend"
	storepkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/store"
	uipkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/ui"
	eventspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/xevents"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	sqlitetool "github.com/go-go-golems/pinocchio/pkg/middlewares/sqlitetool"
	toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	sqlite_regexp "github.com/go-go-golems/go-sqlite-regexp"
	"golang.org/x/sync/errgroup"
)

type SimpleAgentCmd struct{ *cmds.CommandDescription }

func NewSimpleAgentCmd() (*SimpleAgentCmd, error) {
	geLayers, err := geppettolayers.CreateGeppettoLayers()
	if err != nil {
		return nil, err
	}

	desc := cmds.NewCommandDescription(
		"simple-chat-agent",
		cmds.WithShort("Simple streaming chat agent with a calculator tool and a tiny REPL"),
		cmds.WithLayersList(geLayers...),
	)
	return &SimpleAgentCmd{CommandDescription: desc}, nil
}

// Tool implementations moved to pkg/tools

// Generative UI types moved to pkg/tools

// Styling moved to UI/xevents

// Pretty handlers removed (now in pkg/xevents)

// UI forwarder removed (now in pkg/xevents)

// Chat evaluator removed (now in pkg/eval)

// App model removed (now in pkg/ui)

func (c *SimpleAgentCmd) RunIntoWriter(ctx context.Context, parsed *layers.ParsedLayers, _ io.Writer) error {
	// Event router + sink
	router, err := events.NewEventRouter()
	if err != nil {
		return errors.Wrap(err, "router")
	}
	uiCh := make(chan interface{}, 1024)
	eventspkg.AddUIForwarder(router, uiCh)
	// Log tool-call events and info/log events to stdout via zerolog
	router.AddHandler("tool-logger", "chat", func(msg *message.Message) error {
		defer msg.Ack()
		e, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			return err
		}
		switch ev := e.(type) {
		case *events.EventToolCall:
			log.Info().Str("tool", ev.ToolCall.Name).Str("id", ev.ToolCall.ID).Str("input", ev.ToolCall.Input).Msg("ToolCall")
		case *events.EventToolCallExecute:
			log.Info().Str("tool", ev.ToolCall.Name).Str("id", ev.ToolCall.ID).Str("input", ev.ToolCall.Input).Msg("ToolExecute")
		case *events.EventLog:
			l := ev.Level
			if l == "" {
				l = "info"
			}
			log.WithLevel(parseZerologLevel(l)).Str("message", ev.Message).Fields(ev.Fields).Msg("LogEvent")
		case *events.EventInfo:
			log.Info().Str("message", ev.Message).Fields(ev.Data).Msg("InfoEvent")
		}
		return nil
	})
	sink := middleware.NewWatermillSink(router.Publisher, "chat")

	// Engine
	eng, err := factory.NewEngineFromParsedLayers(parsed)
	if err != nil {
		return errors.Wrap(err, "engine")
	}

	// Agent modes for financial analysis and regex design/review
	svc := agentmode.NewStaticService([]*agentmode.AgentMode{
		{Name: "financial_analyst", Prompt: "You are a financial transaction analyst. Your role is to examine transaction data to identify spending patterns, uncover common merchant patterns in descriptions, and discover potential category groupings. Use SQL queries to explore transaction coverage, identify outliers, and find candidates for automatic categorization. Focus on analysis and discovery - do not perform any writes in this mode. Always propose changes with verification queries and explain your reasoning."},
		{Name: "category_regexp_designer", Prompt: "You are a regex pattern designer for transaction categorization. Your job is to create precise regular expressions that match transaction descriptions and automatically assign them to appropriate spending categories. Design minimal, efficient pattern sets that avoid false positives. Always verify your patterns with SQL COUNT(*) queries and sample previews before persisting them with INSERT/UPDATE statements. Focus on accuracy over coverage - it's better to catch fewer transactions correctly than to misclassify many."},
		{Name: "category_regexp_reviewer", Prompt: "You are a pattern review specialist for transaction categorization systems. Your role is to evaluate proposed regex patterns and manual category overrides for accuracy and potential issues. Identify risks such as overmatching (false positives) and undermatching (missed transactions). Suggest improvements to patterns and explain the reasoning behind your recommendations. You are in review-only mode - do not perform any database writes or modifications."},
	})
	amCfg := agentmode.DefaultConfig()
	amCfg.DefaultMode = "financial_analyst"
	// Ensure a consistent system prompt at the start of the Turn
	eng = middleware.NewEngineWithMiddleware(eng,
		middleware.NewSystemPromptMiddleware("You are a financial transaction analysis assistant. Your primary role is to analyze bank transactions and extract spending categories by examining transaction descriptions and developing regular expression patterns to automatically categorize future transactions. You can use various tools to help with data analysis and pattern development."),
		agentmode.NewMiddleware(svc, amCfg),
	)

	// Tools: calculator + generative UI (integrated)
	registry := tools.NewInMemoryToolRegistry()
	if err := toolspkg.RegisterCalculatorTool(registry); err != nil {
		return errors.Wrap(err, "register calc tool")
	}
	// Channel to request UI forms from tools
	toolReqCh := make(chan toolspkg.ToolUIRequest, 4)
	if err := toolspkg.RegisterGenerativeUITool(registry, toolReqCh); err != nil {
		return errors.Wrap(err, "register generative-ui tool")
	}

	// Tools are provided per Turn via registry (handled in evaluator); no engine-level configuration needed

	// State store and stable IDs
	var snapshotStore *storepkg.SQLiteStore
	{
		ss, err := storepkg.NewSQLiteStore("simple-agent.db")
		if err != nil {
			return errors.Wrap(err, "open sqlite store")
		}
		snapshotStore = ss
	}
	sessionRunID := uuid.NewString()

	// Add RW SQLite tool middleware with REGEXP
	dbWithRegexp, _ := sqlite_regexp.OpenWithRegexp("anonymized-data.db")
	eng = middleware.NewEngineWithMiddleware(eng,
		sqlitetool.NewMiddleware(sqlitetool.Config{DB: dbWithRegexp, MaxRows: 500}),
	)

	// Stable IDs + snapshot pre/post middleware
	wrappedEng := middleware.NewEngineWithMiddleware(eng,
		func(next middleware.HandlerFunc) middleware.HandlerFunc {
			return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
				if t == nil {
					t = &turns.Turn{}
				}
				if t.RunID == "" {
					t.RunID = sessionRunID
				}
				if t.ID == "" {
					t.ID = uuid.NewString()
				}
				return next(ctx, t)
			}
		},
		func(next middleware.HandlerFunc) middleware.HandlerFunc {
			return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
				_ = snapshotStore.SaveTurnSnapshot(ctx, t, "pre_middleware")
				res, err := next(ctx, t)
				if res != nil {
					_ = snapshotStore.SaveTurnSnapshot(ctx, res, "post_middleware")
				}
				return res, err
			}
		},
	)

	// Hook for tool loop phases
	hook := func(hctx context.Context, ht *turns.Turn, phase string) {
		_ = snapshotStore.SaveTurnSnapshot(hctx, ht, phase)
	}

	// Backend that runs tool loop
	backend := backendpkg.NewToolLoopBackend(wrappedEng, registry, sink, hook)

	// Chat model using TimelineShell + input, with our renderers
	chatModel := boba_chat.InitialModel(backend,
		boba_chat.WithTitle("Chat REPL"),
		// register only core renderers; backend will add its own via WithTimelineRegister below
		boba_chat.WithTimelineRegister(func(r *timeline.Registry) {
			r.RegisterModelFactory(renderers.NewLLMTextFactory())
			// r.RegisterModelFactory(renderers.ToolCallsPanelFactory{})
			r.RegisterModelFactory(renderers.PlainFactory{})
			r.RegisterModelFactory(renderers.NewToolCallFactory())
			r.RegisterModelFactory(renderers.ToolCallResultFactory{})
			r.RegisterModelFactory(agentmode.AgentModeFactory {})
			r.RegisterModelFactory(renderers.LogEventFactory{})
		}),
	)

	// Wrap chat model with overlay to support generative-ui forms
	app := uipkg.NewOverlayModel(chatModel, toolReqCh)
	// Host model adds sidebar toggle (Ctrl+T) and composes layout around the overlay/chat UI
	host := uipkg.NewHostModel(app, uiCh)

	// Also persist chat events (tool/log/info) into sqlite when received
	router.AddHandler("event-sql-logger", "chat", func(msg *message.Message) error {
		defer msg.Ack()
		e, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			return err
		}
		snapshotStore.LogEvent(ctx, e)
		return nil
	})

	// Run router and Bubble Tea app; cancel router when UI exits
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()
	eg, groupCtx := errgroup.WithContext(ctx2)

	// Build program first so we can register event forwarder before router starts
	p := tea.NewProgram(host, tea.WithAltScreen())
	// Forward geppetto events to timeline UI (agent-specific forwarder, no premature finish)
	router.AddHandler("ui-forward", "chat", backend.MakeUIForwarder(p))

	eg.Go(func() error { return router.Run(groupCtx) })
	eg.Go(func() error {
		_, err := p.Run()
		// Ensure router shuts down when UI exits
		cancel()
		return err
	})
	if err := eg.Wait(); err != nil {
		return err
	}
	log.Info().Msg("Finished")
	return nil
}

func main() {
	root := &cobra.Command{Use: "simple-chat-agent", PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := logging.InitLoggerFromViper(); err != nil {
			return err
		}
		return nil
	}}
	helpSystem := help.NewHelpSystem()
	help_cmd.SetupCobraRootCommand(helpSystem, root)

	if err := clay.InitViper("pinocchio", root); err != nil {
		cobra.CheckErr(err)
	}

	c, err := NewSimpleAgentCmd()
	cobra.CheckErr(err)
	command, err := cli.BuildCobraCommand(c, cli.WithCobraMiddlewaresFunc(geppettolayers.GetCobraCommandGeppettoMiddlewares))
	cobra.CheckErr(err)
	root.AddCommand(command)
	cobra.CheckErr(root.Execute())
}

// parseZerologLevel converts a string level into zerolog.Level with a safe default
func parseZerologLevel(s string) zerolog.Level {
	switch strings.ToLower(s) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	case "info":
		fallthrough
	default:
		return zerolog.InfoLevel
	}
}
