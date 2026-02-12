package main

import (
	"context"
	"fmt"
	"io"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/geppetto/pkg/turns"

	clay "github.com/go-go-golems/clay/pkg"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
)

var rootCmd = &cobra.Command{
	Use:   "simple-redis-streaming-inference",
	Short: "Streaming inference over Redis Streams (Watermill)",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := logging.InitLoggerFromCobra(cmd); err != nil {
			return err
		}
		if f := cmd.Flags(); f != nil {
			lvl, _ := f.GetString("log-level")
			if lvl != "" {
				if l, err := zerolog.ParseLevel(lvl); err == nil {
					zerolog.SetGlobalLevel(l)
				}
			}
			withCaller, _ := f.GetBool("with-caller")
			if withCaller {
				// Enable caller information in logs
				log.Logger = log.Logger.With().Caller().Logger()
			}
		}
		return nil
	},
}

type SimpleRedisStreamingInferenceCommand struct {
	*cmds.CommandDescription
}

var _ cmds.WriterCommand = (*SimpleRedisStreamingInferenceCommand)(nil)

type SimpleRedisStreamingInferenceSettings struct {
	PinocchioProfile string `glazed:"pinocchio-profile"`
	WithLogging      bool   `glazed:"with-logging"`
	WithCaller       bool   `glazed:"with-caller"`
	Prompt           string `glazed:"prompt"`
	OutputFormat     string `glazed:"output-format"`
	WithMetadata     bool   `glazed:"with-metadata"`
	FullOutput       bool   `glazed:"full-output"`
	Verbose          bool   `glazed:"verbose"`

	Redis rediscfg.Settings
}

func NewSimpleRedisStreamingInferenceCommand() (*SimpleRedisStreamingInferenceCommand, error) {
	geLayers, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		return nil, errors.Wrap(err, "create geppetto layers")
	}
	redisLayer, err := rediscfg.NewParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "build redis layer")
	}

	desc := cmds.NewCommandDescription(
		"simple-redis-streaming-inference",
		cmds.WithShort("Simple streaming inference that publishes events to Redis Streams"),
		cmds.WithArguments(
			fields.New("prompt", fields.TypeString, fields.WithHelp("Prompt to run")),
		),
		cmds.WithFlags(
			fields.New("pinocchio-profile", fields.TypeString, fields.WithHelp("Pinocchio profile"), fields.WithDefault("4o-mini")),
			fields.New("with-logging", fields.TypeBool, fields.WithHelp("Enable logging middleware"), fields.WithDefault(false)),
			fields.New("with-caller", fields.TypeBool, fields.WithHelp("Include caller (file:line) in logs"), fields.WithDefault(false)),
			fields.New("output-format", fields.TypeString, fields.WithHelp("Output format (text, json, yaml)"), fields.WithDefault("text")),
			fields.New("with-metadata", fields.TypeBool, fields.WithHelp("Include metadata in output"), fields.WithDefault(false)),
			fields.New("full-output", fields.TypeBool, fields.WithHelp("Include full output details"), fields.WithDefault(false)),
			fields.New("verbose", fields.TypeBool, fields.WithHelp("Verbose event router logging"), fields.WithDefault(false)),
			fields.New("log-level", fields.TypeString, fields.WithHelp("Global log level (trace, debug, info, warn, error)"), fields.WithDefault("")),
		),
		cmds.WithSections(append(geLayers, redisLayer)...),
	)

	return &SimpleRedisStreamingInferenceCommand{CommandDescription: desc}, nil
}

func (c *SimpleRedisStreamingInferenceCommand) RunIntoWriter(ctx context.Context, parsedLayers *values.Values, w io.Writer) error {
	log.Info().Msg("Starting simple Redis streaming inference command")

	s := &SimpleRedisStreamingInferenceSettings{}
	if err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s); err != nil {
		return errors.Wrap(err, "init default settings")
	}
	if err := parsedLayers.DecodeSectionInto("redis", &s.Redis); err != nil {
		return errors.Wrap(err, "init redis settings")
	}

	// Build EventRouter with Redis Streams publisher/subscriber
	log.Info().Str("addr", s.Redis.Addr).Str("group", s.Redis.Group).Str("consumer", s.Redis.Consumer).Msg("Initializing Redis router")
	router, err := rediscfg.BuildRouter(s.Redis, s.Verbose)
	if err != nil {
		return errors.Wrap(err, "create redis event router")
	}
	defer func() { _ = router.Close() }()

	// Create sink publishing to topic "chat"
	sink := middleware.NewWatermillSink(router.Publisher, "chat")
	log.Info().Str("topic", "chat").Msg("Created Watermill sink (publisher -> Redis)")

	// Add a printer handler based on output format
	if s.OutputFormat == "" {
		router.AddHandler("chat", "chat", events.StepPrinterFunc("", w))
	} else {
		printer := events.NewStructuredPrinter(w, events.PrinterOptions{
			Format:          events.PrinterFormat(s.OutputFormat),
			Name:            "",
			IncludeMetadata: s.WithMetadata,
			Full:            s.FullOutput,
		})
		router.AddHandler("chat", "chat", printer)
	}

	// Add a raw dump handler to verify payloads are flowing through Redis
	router.AddHandler("debug-raw", "chat", router.DumpRawEvents)
	// Add a debug event-type handler
	router.AddHandler("debug-events", "chat", func(msg *message.Message) error {
		defer msg.Ack()
		e, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			log.Error().Err(err).Str("payload", string(msg.Payload)).Msg("Failed to parse event JSON")
			return nil
		}
		md := e.Metadata()
		log.Debug().Str("event_type", string(e.Type())).Str("session_id", md.SessionID).Str("inference_id", md.InferenceID).Str("turn_id", md.TurnID).Str("message_id", md.ID.String()).Msg("Received event from Redis stream")
		return nil
	})

	// Build engine and attach the sink via run context.
	eng, err := factory.NewEngineFromParsedValues(parsedLayers)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create engine")
		return errors.Wrap(err, "create engine")
	}
	var mws []middleware.Middleware
	if s.WithLogging {
		mws = append(mws, middleware.NewTurnLoggingMiddleware(log.Logger))
	}

	// Build initial Turn with Blocks
	seed := turns.NewTurnBuilder().
		WithSystemPrompt("You are a helpful assistant. Answer the question in a short and concise manner. ").
		WithUserPrompt(s.Prompt).
		Build()
	sessionID := uuid.NewString()
	if err := turns.KeyTurnMetaSessionID.Set(&seed.Metadata, sessionID); err != nil {
		return fmt.Errorf("set session id metadata: %w", err)
	}

	// Run router and inference concurrently
	eg := errgroup.Group{}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg.Go(func() error {
		defer cancel()
		log.Info().Msg("Starting EventRouter (Redis-backed)")
		return router.Run(ctx)
	})

	var finalTurn *turns.Turn
	eg.Go(func() error {
		defer cancel()
		<-router.Running()
		log.Info().Msg("EventRouter is running; starting inference")
		runner, err := enginebuilder.New(
			enginebuilder.WithBase(eng),
			enginebuilder.WithMiddlewares(mws...),
			enginebuilder.WithEventSinks(sink),
		).Build(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("failed to build runner: %w", err)
		}
		updatedTurn, err := runner.RunInference(ctx, seed)
		if err != nil {
			log.Error().Err(err).Msg("Inference failed")
			return fmt.Errorf("inference failed: %w", err)
		}
		finalTurn = updatedTurn
		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	fmt.Fprintln(w, "\n=== Final Turn ===")
	if finalTurn != nil {
		turns.FprintTurn(w, finalTurn)
	}

	log.Info().Msg("Simple Redis streaming inference command completed successfully")
	return nil
}

func main() {
	if err := clay.InitGlazed("pinocchio", rootCmd); err != nil {
		cobra.CheckErr(err)
	}

	helpSystem := help.NewHelpSystem()
	help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

	c, err := NewSimpleRedisStreamingInferenceCommand()
	cobra.CheckErr(err)

	command, err := cli.BuildCobraCommand(c, cli.WithCobraMiddlewaresFunc(geppettosections.GetCobraCommandGeppettoMiddlewares))
	cobra.CheckErr(err)
	rootCmd.AddCommand(command)

	cobra.CheckErr(rootCmd.Execute())
}
