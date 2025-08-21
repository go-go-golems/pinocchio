package main

import (
    "context"
    "embed"
    "encoding/json"
    "io"
    "net/http"
    "sync"

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
    router, err := rediscfg.BuildRouter(rs, true)
    if err != nil {
        return errors.Wrap(err, "build router")
    }
    defer func() { _ = router.Close() }()

    // Run router so in-memory transport works and Redis handlers are active
    eg := errgroup.Group{}
    srvCtx, srvCancel := context.WithCancel(ctx)
    defer srvCancel()
    eg.Go(func() error { return router.Run(srvCtx) })

    // Conversation and run management
    type Conversation struct {
        ID       string
        RunID    string
        running  bool
        cancel   context.CancelFunc
        mu       sync.Mutex
        conns    map[*websocket.Conn]bool
        connsMu  sync.RWMutex
        sub      message.Subscriber
        stopRead context.CancelFunc
    }
    type ConvManager struct {
        mu    sync.Mutex
        conns map[string]*Conversation
    }
    cm := &ConvManager{conns: make(map[string]*Conversation)}
    getOrCreateConv := func(convID string) (*Conversation, error) {
        cm.mu.Lock()
        defer cm.mu.Unlock()
        if conv, ok := cm.conns[convID]; ok {
            return conv, nil
        }
        runID := uuid.NewString()
        conv := &Conversation{ID: convID, RunID: runID, conns: make(map[*websocket.Conn]bool)}
        // Create dedicated subscriber per conversation
        if rs.Enabled {
            s_, err := rediscfg.BuildGroupSubscriber(rs.Addr, "ui-"+convID, "web-"+convID)
            if err != nil {
                return nil, err
            }
            conv.sub = s_
        } else {
            conv.sub = router.Subscriber
        }
        // Start reader goroutine for this conversation
        readCtx, readCancel := context.WithCancel(srvCtx)
        conv.stopRead = readCancel
        ch, err := conv.sub.Subscribe(readCtx, "chat")
        if err != nil {
            readCancel()
            return nil, err
        }
        go func() {
            for msg := range ch {
                e, err := events.NewEventFromJson(msg.Payload)
                if err != nil { msg.Ack(); continue }
                if e.Metadata().RunID != conv.RunID { msg.Ack(); continue }
                conv.connsMu.RLock()
                for c := range conv.conns {
                    _ = c.WriteMessage(websocket.TextMessage, msg.Payload)
                }
                conv.connsMu.RUnlock()
                msg.Ack()
            }
        }()
        cm.conns[convID] = conv
        return conv, nil
    }
    addConn := func(conv *Conversation, c *websocket.Conn) {
        conv.connsMu.Lock()
        conv.conns[c] = true
        conv.connsMu.Unlock()
    }
    removeConn := func(conv *Conversation, c *websocket.Conn) {
        conv.connsMu.Lock()
        delete(conv.conns, c)
        conv.connsMu.Unlock()
        _ = c.Close()
    }

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
        convID := r.URL.Query().Get("conv_id")
        if convID == "" {
            // no conv_id; close
            _ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"missing conv_id"}`))
            _ = conn.Close()
            return
        }
        conv, err := getOrCreateConv(convID)
        if err != nil {
            _ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"failed to join conversation"}`))
            _ = conn.Close()
            return
        }
        addConn(conv, conn)
        // keep connection open until client closes
        go func() {
            defer removeConn(conv, conn)
            for {
                if _, _, err := conn.ReadMessage(); err != nil { return }
            }
        }()
    })

    // Start a chat turn by POSTing {"prompt":"..."}. Returns {"run_id":"..."}.
    http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        var body struct{ Prompt string `json:"prompt"`; ConvID string `json:"conv_id"` }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }
        // Conversation lookup or creation
        convID := body.ConvID
        if convID == "" { convID = uuid.NewString() }
        conv, err := getOrCreateConv(convID)
        if err != nil {
            http.Error(w, "failed to create conversation", http.StatusInternalServerError)
            return
        }
        conv.mu.Lock()
        if conv.running {
            conv.mu.Unlock()
            w.WriteHeader(http.StatusConflict)
            _ = json.NewEncoder(w).Encode(map[string]any{"error":"run in progress","conv_id": conv.ID, "run_id": conv.RunID})
            return
        }
        conv.running = true
        conv.mu.Unlock()

        // Create sink publishing to topic "chat"
        sink := middleware.NewWatermillSink(router.Publisher, "chat")

        // Build engine from parsed layers
        eng, err := factory.NewEngineFromParsedLayers(parsed, engine.WithSink(sink))
        if err != nil {
            http.Error(w, "engine init failed", http.StatusInternalServerError)
            return
        }

        // Prepare turn with a fresh run_id
        runID := conv.RunID
        seed := turns.NewTurnBuilder().WithUserPrompt(body.Prompt).Build()
        seed.RunID = runID

        go func(runID string, conv *Conversation) {
            // Wait for router to be running for delivery
            <-router.Running()
            // Run in a background context independent of request lifetime
            runCtx, runCancel := context.WithCancel(srvCtx)
            conv.mu.Lock(); conv.cancel = runCancel; conv.mu.Unlock()
            _, _ = eng.RunInference(runCtx, seed)
            runCancel()
            conv.mu.Lock(); conv.running = false; conv.cancel = nil; conv.mu.Unlock()
        }(runID, conv)

        _ = json.NewEncoder(w).Encode(map[string]string{"run_id": runID, "conv_id": conv.ID})
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


