package main

import (
    "context"
    "embed"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/ThreeDotsLabs/watermill/message"
    "github.com/gorilla/websocket"

    clay "github.com/go-go-golems/clay/pkg"
    "github.com/go-go-golems/geppetto/pkg/events"
    geppettolayers "github.com/go-go-golems/geppetto/pkg/layers"
    "github.com/go-go-golems/geppetto/pkg/inference/engine"
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/turns"
    rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
    "github.com/go-go-golems/glazed/pkg/cli"
    "github.com/go-go-golems/glazed/pkg/cmds"
    "github.com/go-go-golems/glazed/pkg/cmds/layers"
    "github.com/go-go-golems/glazed/pkg/cmds/parameters"
    "github.com/google/uuid"
    "github.com/pkg/errors"
    "github.com/rs/zerolog/log"
    "golang.org/x/sync/errgroup"
)

//go:embed static/index.html
var indexHTML embed.FS

// no package-level root; we will build a cobra command dynamically in main()

type WebServerSettings struct {
    Addr string `glazed.parameter:"addr"`
}

type Command struct {
    *cmds.CommandDescription
}

func NewCommand() (*Command, error) {
    geLayers, err := geppettolayers.CreateGeppettoLayers()
    if err != nil {
        return nil, errors.Wrap(err, "create geppetto layers")
    }
    redisLayer, err := rediscfg.NewParameterLayer()
    if err != nil {
        return nil, err
    }

    desc := cmds.NewCommandDescription(
        "web-chat",
        cmds.WithShort("Serve a minimal WebSocket web UI that streams chat events"),
        cmds.WithFlags(
            parameters.NewParameterDefinition("addr", parameters.ParameterTypeString, parameters.WithDefault(":8080"), parameters.WithHelp("HTTP listen address")),
        ),
        cmds.WithLayersList(append(geLayers, redisLayer)...),
    )
    return &Command{CommandDescription: desc}, nil
}

func (c *Command) RunIntoWriter(ctx context.Context, parsed *layers.ParsedLayers, _ io.Writer) error {
    s := &WebServerSettings{}
    if err := parsed.InitializeStruct(layers.DefaultSlug, s); err != nil {
        return errors.Wrap(err, "init server settings")
    }

    rs := rediscfg.Settings{}
    _ = parsed.InitializeStruct("redis", &rs)

    // Build router to obtain default subscriber for in-memory fallback, though we will prefer per-connection subscribers when Redis is enabled.
    router, err := rediscfg.BuildRouter(rs, false)
    if err != nil {
        return errors.Wrap(err, "build router")
    }
    defer func() { _ = router.Close() }()

    // Run router so in-memory transport works and Redis handlers are active
    eg := errgroup.Group{}
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()
    eg.Go(func() error { return router.Run(ctx) })

    upgrader := websocket.Upgrader{ CheckOrigin: func(r *http.Request) bool { return true } }

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        b, err := indexHTML.ReadFile("static/index.html")
        if err != nil {
            http.Error(w, "index not found", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        _, _ = w.Write(b)
    })

    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            log.Error().Err(err).Msg("websocket upgrade failed")
            return
        }

        // Unique consumer name per connection when using Redis.
        consumerName := fmt.Sprintf("ui-%d", time.Now().UnixNano())
        var sub message.Subscriber
        if rs.Enabled {
            s_, err := rediscfg.BuildGroupSubscriber(rs.Addr, "ui", consumerName)
            if err != nil {
                log.Error().Err(err).Msg("failed to build group subscriber")
                _ = conn.Close()
                return
            }
            sub = s_
        } else {
            sub = router.Subscriber
        }

        ctxConn, cancel := context.WithCancel(r.Context())

        topic := "chat"
        ch, err := sub.Subscribe(ctxConn, topic)
        if err != nil {
            log.Error().Err(err).Str("topic", topic).Msg("subscribe failed")
            cancel()
            _ = conn.Close()
            return
        }

        runID := r.URL.Query().Get("run_id")

        go func() {
            defer func() {
                cancel()
                _ = conn.Close()
            }()
            for msg := range ch {
                e, err := events.NewEventFromJson(msg.Payload)
                if err != nil {
                    msg.Ack()
                    continue
                }
                if runID != "" && e.Metadata().RunID != runID {
                    msg.Ack()
                    continue
                }
                if err := conn.WriteMessage(websocket.TextMessage, msg.Payload); err != nil {
                    msg.Ack()
                    return
                }
                msg.Ack()
            }
        }()
    })

    // Start a chat turn by POSTing {"prompt":"..."}. Returns {"run_id":"..."}.
    http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        var body struct{ Prompt string `json:"prompt"` }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }

        // Create sink publishing to topic "chat"
        sink := middleware.NewWatermillSink(router.Publisher, "chat")

        // Build engine from parsed layers
        eng, err := factory.NewEngineFromParsedLayers(parsed, engine.WithSink(sink))
        if err != nil {
            http.Error(w, "engine init failed", http.StatusInternalServerError)
            return
        }

        // Prepare turn with a fresh run_id
        runID := uuid.NewString()
        seed := turns.NewTurnBuilder().WithUserPrompt(body.Prompt).Build()
        seed.RunID = runID

        go func() {
            // Wait for router to be running for Redis delivery
            <-router.Running()
            _, _ = eng.RunInference(r.Context(), seed)
        }()

        _ = json.NewEncoder(w).Encode(map[string]string{"run_id": runID})
    })

    log.Info().Str("addr", s.Addr).Msg("starting web-chat server")
    if err := http.ListenAndServe(s.Addr, nil); err != nil {
        return err
    }
    return eg.Wait()
}

func main() {
    cmd, err := NewCommand()
    if err != nil {
        panic(err)
    }
    cobraCmd, err := cli.BuildCobraCommand(cmd, cli.WithCobraMiddlewaresFunc(geppettolayers.GetCobraCommandGeppettoMiddlewares))
    if err != nil {
        panic(err)
    }
    // Initialize viper and logging on the built root command
    if err := clay.InitViper("pinocchio", cobraCmd); err != nil {
        panic(err)
    }
    // Ensure cobra has a Use if not set
    if cobraCmd.Use == "" {
        cobraCmd.Use = "web-chat"
    }
    if err := cobraCmd.Execute(); err != nil {
        panic(err)
    }
}


