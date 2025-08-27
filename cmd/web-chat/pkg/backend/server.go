package backend

import (
    "context"
    "database/sql"
    "embed"
    "encoding/json"
    "io/fs"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gorilla/websocket"
    "github.com/google/uuid"
    "github.com/pkg/errors"
    "github.com/rs/zerolog/log"
    "golang.org/x/sync/errgroup"

    "github.com/go-go-golems/geppetto/pkg/events"
    "github.com/go-go-golems/geppetto/pkg/inference/engine"
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/inference/toolhelpers"
    geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
    "github.com/go-go-golems/geppetto/pkg/turns"
    "github.com/go-go-golems/glazed/pkg/cmds/layers"

    toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
    agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
    sqlitetool "github.com/go-go-golems/pinocchio/pkg/middlewares/sqlitetool"
    rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
)

// WebServerSettings controls the embedded HTTP server.
type WebServerSettings struct {
    Addr               string `glazed.parameter:"addr"`
    AgentModeEnabled   bool   `glazed.parameter:"enable-agentmode"`
    IdleTimeoutSeconds int    `glazed.parameter:"idle-timeout-seconds"`
}

// Server owns HTTP handlers, conversation state and streaming pipeline.
type Server struct {
    baseCtx  context.Context
    settings WebServerSettings
    redis    rediscfg.Settings

    parsedLayers *layers.ParsedLayers

    router  *events.EventRouter
    mux     *http.ServeMux
    server  *http.Server

    staticFS embed.FS

    // shared registries/deps
    registry geptools.ToolRegistry
    db       *sql.DB

    // agent mode
    amSvc *agentmode.StaticService
    amCfg agentmode.Config

    // websocket upgrader
    upgrader websocket.Upgrader

    // conversation manager
    cm *ConvManager
}

// NewServer constructs a Server and registers HTTP routes on a provided mux.
func NewServer(ctx context.Context, parsed *layers.ParsedLayers, staticFS embed.FS) (*Server, error) {
    // Load CLI/server settings from layers
    s := &WebServerSettings{}
    if err := parsed.InitializeStruct(layers.DefaultSlug, s); err != nil {
        return nil, errors.Wrap(err, "init server settings")
    }
    rs := rediscfg.Settings{}
    _ = parsed.InitializeStruct("redis", &rs)

    // Build event router (in-memory or Redis-backed)
    router, err := rediscfg.BuildRouter(rs, true)
    if err != nil {
        return nil, errors.Wrap(err, "build router")
    }

    // Shared tool registry
    registry := geptools.NewInMemoryToolRegistry()
    _ = toolspkg.RegisterCalculatorTool(registry)

    // Optional SQLite DB (best-effort)
    var dbWithRegexp *sql.DB
    if db, err := sql.Open("sqlite3", "anonymized-data.db"); err == nil {
        dbWithRegexp = db
        log.Info().Str("dsn", "anonymized-data.db").Msg("opened sqlite database")
    } else {
        log.Warn().Err(err).Msg("could not open sqlite DB; SQL tool middleware disabled")
    }

    // Agent mode configuration (optional)
    amSvc := agentmode.NewStaticService([]*agentmode.AgentMode{
        {Name: "financial_analyst", Prompt: "You are a financial transaction analyst. Analyze transactions and propose categories."},
        {Name: "category_regexp_designer", Prompt: "Design regex patterns to categorize transactions. Verify with SQL counts before proposing changes."},
        {Name: "category_regexp_reviewer", Prompt: "Review proposed regex patterns and assess over/under matching risks."},
    })
    amCfg := agentmode.DefaultConfig()
    amCfg.DefaultMode = "financial_analyst"

    srv := &Server{
        baseCtx:      ctx,
        settings:     *s,
        redis:        rs,
        parsedLayers: parsed,
        router:       router,
        mux:          http.NewServeMux(),
        staticFS:     staticFS,
        registry:     registry,
        db:           dbWithRegexp,
        amSvc:        amSvc,
        amCfg:        amCfg,
        upgrader:     websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
        cm:           &ConvManager{conns: make(map[string]*Conversation)},
    }

    srv.registerHTTPHandlers()

    // Build HTTP server
    srv.server = &http.Server{
        Addr:              s.Addr,
        Handler:           srv.mux,
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       30 * time.Second,
        WriteTimeout:      60 * time.Second,
        IdleTimeout:       120 * time.Second,
    }

    return srv, nil
}

// Run starts the event router and HTTP server and blocks until shutdown.
func (s *Server) Run(ctx context.Context) error {
    eg := errgroup.Group{}
    srvCtx, srvCancel := context.WithCancel(ctx)
    defer srvCancel()

    // Start event router (supports in-memory or Redis streams)
    eg.Go(func() error { return s.router.Run(srvCtx) })

    // Signal handling and graceful shutdown
    eg.Go(func() error {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
        <-sigChan
        log.Info().Msg("received interrupt signal, shutting down gracefully...")
        srvCancel()
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        if err := s.server.Shutdown(shutdownCtx); err != nil {
            log.Error().Err(err).Msg("server shutdown error")
            return err
        }
        if err := s.router.Close(); err != nil {
            log.Error().Err(err).Msg("router close error")
        } else {
            log.Info().Msg("router closed")
        }
        log.Info().Msg("server shutdown complete")
        return nil
    })

    // Start HTTP server
    eg.Go(func() error {
        log.Info().Str("addr", s.settings.Addr).Msg("starting web-chat server")
        if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Error().Err(err).Msg("server listen error")
            return err
        }
        return nil
    })

    return eg.Wait()
}

// helper to create per-conversation topic
func (s *Server) topicForConv(convID string) string {
    return "chat:" + convID
}

// buildEngine builds a per-conversation engine with configured middlewares.
func (s *Server) buildEngine() (engine.Engine, error) {
    eng, err := factory.NewEngineFromParsedLayers(s.parsedLayers)
    if err != nil {
        return nil, errors.Wrap(err, "engine init failed")
    }
    eng = middleware.NewEngineWithMiddleware(eng, middleware.NewSystemPromptMiddleware("You are a helpful assistant. Be concise."))
    if s.settings.AgentModeEnabled {
        eng = middleware.NewEngineWithMiddleware(eng, agentmode.NewMiddleware(s.amSvc, s.amCfg))
    }
    eng = middleware.NewEngineWithMiddleware(eng, middleware.NewToolResultReorderMiddleware())
    if s.db != nil {
        eng = middleware.NewEngineWithMiddleware(eng, sqlitetool.NewMiddleware(sqlitetool.Config{DB: s.db, MaxRows: 500}))
    }
    return eng, nil
}

// registerHTTPHandlers mounts static assets and API endpoints on the server mux.
func (s *Server) registerHTTPHandlers() {
    // static assets
    if staticSub, err := fs.Sub(s.staticFS, "static"); err == nil {
        s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
    }
    // built dist assets (vite)
    if distAssets, err := fs.Sub(s.staticFS, "static/dist/assets"); err == nil {
        s.mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(distAssets))))
    }
    // index
    s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        b, err := s.staticFS.ReadFile("static/dist/index.html")
        if err != nil {
            b, err = s.staticFS.ReadFile("static/index.html")
            if err != nil {
                http.Error(w, "index not found", http.StatusInternalServerError)
                return
            }
        }
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        _, _ = w.Write(b)
    })

    // websocket endpoint
    s.mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, err := s.upgrader.Upgrade(w, r, nil)
        if err != nil {
            log.Error().Err(err).Msg("websocket upgrade failed")
            return
        }
        convID := r.URL.Query().Get("conv_id")
        if convID == "" {
            _ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"missing conv_id"}`))
            _ = conn.Close()
            return
        }
        conv, err := s.getOrCreateConv(convID)
        if err != nil {
            _ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"failed to join conversation"}`))
            _ = conn.Close()
            return
        }
        s.addConn(conv, conn)
        go func() {
            defer s.removeConn(conv, conn)
            for {
                if _, _, err := conn.ReadMessage(); err != nil {
                    return
                }
            }
        }()
    })

    // start chat run
    s.mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        var body struct {
            Prompt string `json:"prompt"`
            ConvID string `json:"conv_id"`
        }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }
        convID := body.ConvID
        if convID == "" {
            convID = uuid.NewString()
        }
        conv, err := s.getOrCreateConv(convID)
        if err != nil {
            http.Error(w, "failed to create conversation", http.StatusInternalServerError)
            return
        }
        conv.mu.Lock()
        if conv.running {
            conv.mu.Unlock()
            w.WriteHeader(http.StatusConflict)
            _ = json.NewEncoder(w).Encode(map[string]any{"error": "run in progress", "conv_id": conv.ID, "run_id": conv.RunID})
            return
        }
        conv.running = true
        conv.mu.Unlock()

        if conv.Turn == nil {
            conv.Turn = &turns.Turn{RunID: conv.RunID, Data: map[string]any{}}
        }
        turns.AppendBlock(conv.Turn, turns.NewUserTextBlock(body.Prompt))
        conv.Turn.RunID = conv.RunID

        go func(conv *Conversation) {
            <-s.router.Running()
            runCtx, runCancel := context.WithCancel(s.baseCtx)
            conv.mu.Lock()
            conv.cancel = runCancel
            conv.mu.Unlock()
            runCtx = events.WithEventSinks(runCtx, conv.Sink)
            if conv.Turn.Data == nil {
                conv.Turn.Data = map[string]any{}
            }
            updatedTurn, _ := toolhelpers.RunToolCallingLoop(
                runCtx,
                conv.Eng,
                conv.Turn,
                s.registry,
                toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60*time.Second),
            )
            if updatedTurn != nil {
                conv.Turn = updatedTurn
            }
            runCancel()
            conv.mu.Lock()
            conv.running = false
            conv.cancel = nil
            conv.mu.Unlock()
        }(conv)

        _ = json.NewEncoder(w).Encode(map[string]string{"run_id": conv.RunID, "conv_id": conv.ID})
    })
}


