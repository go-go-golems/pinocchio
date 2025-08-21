package main

import (
    "context"
    "embed"
    "encoding/json"
    "io"
    "io/fs"
    "net/http"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "github.com/ThreeDotsLabs/watermill/message"
    "github.com/gorilla/websocket"

    clay "github.com/go-go-golems/clay/pkg"
    "github.com/go-go-golems/geppetto/pkg/events"
    geppettolayers "github.com/go-go-golems/geppetto/pkg/layers"
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/inference/toolhelpers"
    geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
    "github.com/go-go-golems/geppetto/pkg/turns"
    rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
    webbackend "github.com/go-go-golems/pinocchio/cmd/web-chat/pkg/backend"
    agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
    sqlitetool "github.com/go-go-golems/pinocchio/pkg/middlewares/sqlitetool"
    toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
    "database/sql"
    "strings"
    "github.com/rs/zerolog"
    "github.com/go-go-golems/glazed/pkg/cli"
    "github.com/go-go-golems/glazed/pkg/cmds"
    "github.com/go-go-golems/glazed/pkg/cmds/logging"
    "github.com/go-go-golems/glazed/pkg/cmds/layers"
    "github.com/go-go-golems/glazed/pkg/cmds/parameters"
    "github.com/go-go-golems/glazed/pkg/help"
    help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
    "github.com/google/uuid"
    "github.com/pkg/errors"
    "github.com/rs/zerolog/log"
    "github.com/spf13/cobra"
    "golang.org/x/sync/errgroup"
)

//go:embed static/*
var staticFS embed.FS

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
    // Router will be closed on shutdown signal

    // Run router so in-memory transport works and Redis handlers are active
    eg := errgroup.Group{}
    srvCtx, srvCancel := context.WithCancel(ctx)
    defer srvCancel()
    eg.Go(func() error { return router.Run(srvCtx) })
    // Shared tool registry and optional DB for SQL middleware
    registry := geptools.NewInMemoryToolRegistry()
    _ = toolspkg.RegisterCalculatorTool(registry)
    // Generative UI tool could be integrated with a web form channel in the future
    // Open SQLite DB with REGEXP support (optional)
    var dbWithRegexp *sql.DB
    {
        // Open a standard SQLite DB; if REGEXP is needed, ensure build tags enable it or use a helper
        dsn := "anonymized-data.db"
        if db, err := sql.Open("sqlite3", dsn); err == nil {
            dbWithRegexp = db
            log.Info().Str("dsn", dsn).Msg("opened sqlite database")
        } else {
            log.Warn().Err(err).Msg("could not open sqlite DB; SQL tool middleware disabled")
        }
    }

    // Agent mode static service and config (optional)
    amSvc := agentmode.NewStaticService([]*agentmode.AgentMode{
        {Name: "financial_analyst", Prompt: "You are a financial transaction analyst. Analyze transactions and propose categories."},
        {Name: "category_regexp_designer", Prompt: "Design regex patterns to categorize transactions. Verify with SQL counts before proposing changes."},
        {Name: "category_regexp_reviewer", Prompt: "Review proposed regex patterns and assess over/under matching risks."},
    })
    amCfg := agentmode.DefaultConfig()
    amCfg.DefaultMode = "financial_analyst"

    // Logging handler for tool/info events; use separate group when Redis enabled
    logHandler := func(msg *message.Message) error {
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
            lvl := ev.Level
            if lvl == "" { lvl = "info" }
            log.WithLevel(parseZerologLevel(lvl)).Str("message", ev.Message).Fields(ev.Fields).Msg("LogEvent")
        case *events.EventInfo:
            log.Info().Str("message", ev.Message).Fields(ev.Data).Msg("InfoEvent")
        }
        return nil
    }
    if rs.Enabled {
        if sub, err := rediscfg.BuildGroupSubscriber(rs.Addr, "logs", "logs-web-1"); err == nil {
            router.AddHandlerWithOptions("tool-logger", "chat", logHandler, events.WithHandlerSubscriber(sub))
        } else {
            router.AddHandler("tool-logger", "chat", logHandler)
        }
    } else {
        router.AddHandler("tool-logger", "chat", logHandler)
    }


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
        // Helper: convert geppetto events to timeline lifecycle messages and broadcast
        convertAndBroadcast := func(e events.Event) {
            sendBytes := func(b []byte) {
                conv.connsMu.RLock()
                for c := range conv.conns { _ = c.WriteMessage(websocket.TextMessage, b) }
                conv.connsMu.RUnlock()
            }
            if bs := webbackend.TimelineEventsFromEvent(e); bs != nil {
                for _, b := range bs { sendBytes(b) }
            }
        }

        go func() {
            for msg := range ch {
                e, err := events.NewEventFromJson(msg.Payload)
                if err != nil { msg.Ack(); continue }
                if e.Metadata().RunID != conv.RunID { msg.Ack(); continue }
                convertAndBroadcast(e)
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

    // static assets
    staticSub, err := fs.Sub(staticFS, "static")
    if err != nil {
        return err
    }
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        b, err := staticFS.ReadFile("static/index.html")
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
        eng, err := factory.NewEngineFromParsedLayers(parsed)
        if err != nil {
            http.Error(w, "engine init failed", http.StatusInternalServerError)
            return
        }
        // Compose middlewares similar to simple-chat-agent
        // Ensure a consistent system prompt + agent mode + tool result reordering
        eng = middleware.NewEngineWithMiddleware(eng,
            middleware.NewSystemPromptMiddleware("You are a helpful assistant. Be concise."),
            agentmode.NewMiddleware(amSvc, amCfg),
            middleware.NewToolResultReorderMiddleware(),
        )
        // Optional: SQL tool middleware if DB available
        if dbWithRegexp != nil {
            eng = middleware.NewEngineWithMiddleware(eng, sqlitetool.NewMiddleware(sqlitetool.Config{DB: dbWithRegexp, MaxRows: 500}))
        }

        // Prepare turn with a fresh run_id
        runID := conv.RunID
        seed := turns.NewTurnBuilder().WithUserPrompt(body.Prompt).Build()
        seed.RunID = runID

        // Broadcast user entity to connected clients (so frontend shows user message via WS)
        userMsg := map[string]any{"type": "user", "text": body.Prompt, "conv_id": conv.ID, "run_id": runID}
        userPayload, _ := json.Marshal(userMsg)
        conv.connsMu.RLock()
        for c := range conv.conns {
            _ = c.WriteMessage(websocket.TextMessage, userPayload)
        }
        conv.connsMu.RUnlock()

        go func(runID string, conv *Conversation) {
            // Wait for router to be running for delivery
            <-router.Running()
            // Run in a background context independent of request lifetime
            runCtx, runCancel := context.WithCancel(srvCtx)
            conv.mu.Lock(); conv.cancel = runCancel; conv.mu.Unlock()
            // Run tool loop with sink attached to context
            runCtx = events.WithEventSinks(runCtx, sink)
            // Ensure seed turn has Data map initialized to avoid nil map assignments in helpers
            if seed.Data == nil { seed.Data = map[string]any{} }
            _, _ = toolhelpers.RunToolCallingLoop(
                runCtx,
                eng,
                seed,
                registry,
                toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60*time.Second),
            )
            runCancel()
            conv.mu.Lock(); conv.running = false; conv.cancel = nil; conv.mu.Unlock()
        }(runID, conv)

        _ = json.NewEncoder(w).Encode(map[string]string{"run_id": runID, "conv_id": conv.ID})
    })

    // Create HTTP server
    server := &http.Server{
        Addr: s.Addr,
    }

    // Handle graceful shutdown
    eg.Go(func() error {
        // Wait for interrupt signal
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
        <-sigChan
        
        log.Info().Msg("received interrupt signal, shutting down gracefully...")
        
        // Cancel server context first to stop router and background tasks
        srvCancel()
        
        // Shutdown HTTP server with timeout
        shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer shutdownCancel()
        
        if err := server.Shutdown(shutdownCtx); err != nil {
            log.Error().Err(err).Msg("server shutdown error")
            return err
        }

        // Close event router (publishers/subscribers and watermill router)
        if err := router.Close(); err != nil {
            log.Error().Err(err).Msg("router close error")
        } else {
            log.Info().Msg("router closed")
        }
        
        log.Info().Msg("server shutdown complete")
        return nil
    })

    // Start HTTP server
    eg.Go(func() error {
        log.Info().Str("addr", s.Addr).Msg("starting web-chat server")
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Error().Err(err).Msg("server listen error")
            return err
        }
        return nil
    })

    return eg.Wait()
}

func main() {
    root := &cobra.Command{Use: "web-chat", PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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

    c, err := NewCommand()
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


