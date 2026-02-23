package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	geppettomw "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
	thinkingmode "github.com/go-go-golems/pinocchio/cmd/web-chat/thinkingmode"
	timelinecmd "github.com/go-go-golems/pinocchio/cmd/web-chat/timeline"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	sqlitetool "github.com/go-go-golems/pinocchio/pkg/middlewares/sqlitetool"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

//go:embed static
var staticFS embed.FS

// no package-level root; we will build a cobra command dynamically in main()

type Command struct {
	*cmds.CommandDescription
}

type webChatRuntimeConfig struct {
	BasePrefix      string `json:"basePrefix"`
	DebugAPIEnabled bool   `json:"debugApiEnabled"`
}

func normalizeBasePrefix(prefix string) string {
	p := strings.TrimSpace(prefix)
	if p == "" || p == "/" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(p, "/")
}

func runtimeConfigScript(basePrefix string, debugAPI bool) (string, error) {
	payload, err := json.Marshal(webChatRuntimeConfig{
		BasePrefix:      normalizeBasePrefix(basePrefix),
		DebugAPIEnabled: debugAPI,
	})
	if err != nil {
		return "", err
	}
	return "window.__PINOCCHIO_WEBCHAT_CONFIG__ = " + string(payload) + ";\n", nil
}

func NewCommand() (*Command, error) {
	geLayers, err := geppettosections.CreateGeppettoSections()
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
			fields.New("addr", fields.TypeString, fields.WithDefault(":8080"), fields.WithHelp("HTTP listen address")),
			fields.New("enable-agentmode", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Enable agent mode middleware")),
			fields.New("idle-timeout-seconds", fields.TypeInteger, fields.WithDefault(60), fields.WithHelp("Stop per-conversation reader after N seconds with no sockets (0=disabled)")),
			fields.New("evict-idle-seconds", fields.TypeInteger, fields.WithDefault(300), fields.WithHelp("Evict conversations after N seconds idle (0=disabled)")),
			fields.New("evict-interval-seconds", fields.TypeInteger, fields.WithDefault(60), fields.WithHelp("Sweep idle conversations every N seconds (0=disabled)")),
			fields.New("root", fields.TypeString, fields.WithDefault("/"), fields.WithHelp("Serve the chat UI under a given URL root (e.g., /chat)")),
			fields.New("debug-api", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Enable debug API endpoints under /api/debug/*")),
			fields.New("timeline-dsn", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DSN for durable timeline snapshots (enables GET /timeline); preferred over timeline-db")),
			fields.New("timeline-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DB file path for durable timeline snapshots (enables GET /timeline); DSN is derived with WAL/busy_timeout")),
			fields.New("turns-dsn", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DSN for durable turn snapshots (enables GET /turns); preferred over turns-db")),
			fields.New("turns-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DB file path for durable turn snapshots (enables GET /turns); DSN is derived with WAL/busy_timeout")),
		),
		cmds.WithSections(append(geLayers, redisLayer)...),
	)
	return &Command{CommandDescription: desc}, nil
}

func (c *Command) RunIntoWriter(ctx context.Context, parsed *values.Values, _ io.Writer) error {
	type serverSettings struct {
		Root     string `glazed:"root"`
		DebugAPI bool   `glazed:"debug-api"`
	}
	s := &serverSettings{}
	if err := parsed.DecodeSectionInto(values.DefaultSlug, s); err != nil {
		return errors.Wrap(err, "decode server settings")
	}
	appConfigJS, err := runtimeConfigScript(s.Root, s.DebugAPI)
	if err != nil {
		return errors.Wrap(err, "build runtime config script")
	}

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

	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{
			Slug: gepprofiles.MustProfileSlug("default"),
			Runtime: gepprofiles.RuntimeSpec{
				SystemPrompt: "You are an assistant",
				Middlewares:  []gepprofiles.MiddlewareUse{},
			},
		},
		&gepprofiles.Profile{
			Slug: gepprofiles.MustProfileSlug("agent"),
			Runtime: gepprofiles.RuntimeSpec{
				SystemPrompt: "You are a helpful assistant. Be concise.",
				Middlewares:  []gepprofiles.MiddlewareUse{{Name: "agentmode", Config: amCfg}},
			},
			Policy: gepprofiles.PolicySpec{
				AllowOverrides: true,
			},
		},
	)
	if err != nil {
		return errors.Wrap(err, "initialize profile registry")
	}

	middlewareFactories := map[string]infruntime.MiddlewareBuilder{
		"agentmode": func(cfg any) geppettomw.Middleware {
			return agentmode.NewMiddleware(amSvc, cfg.(agentmode.Config))
		},
		"sqlite": func(cfg any) geppettomw.Middleware {
			c := sqlitetool.Config{DB: dbWithRegexp}
			if cfg_, ok := cfg.(sqlitetool.Config); ok {
				c = cfg_
			}
			return sqlitetool.NewMiddleware(c)
		},
	}
	runtimeComposer := newProfileRuntimeComposer(parsed, middlewareFactories)
	requestResolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultRegistrySlug))

	// Register app-owned thinking-mode SEM/timeline handlers.
	thinkingmode.Register()

	// Build webchat server and register middlewares/tools/profile handlers.
	srv, err := webchat.NewServer(
		ctx,
		parsed,
		staticFS,
		webchat.WithRuntimeComposer(runtimeComposer),
		webchat.WithDebugRoutesEnabled(s.DebugAPI),
	)
	if err != nil {
		return errors.Wrap(err, "new webchat server")
	}

	// Register middlewares
	for name, factory := range middlewareFactories {
		srv.RegisterMiddleware(name, factory)
	}

	// Register calculator tool
	srv.RegisterTool("calculator", func(reg geptools.ToolRegistry) error {
		if im, ok := reg.(*geptools.InMemoryToolRegistry); ok {
			return toolspkg.RegisterCalculatorTool(im)
		}
		im2 := geptools.NewInMemoryToolRegistry()
		if err := toolspkg.RegisterCalculatorTool(im2); err != nil {
			return err
		}
		for _, td := range im2.ListTools() {
			_ = reg.RegisterTool(td.Name, td)
		}
		return nil
	})

	chatHandler := webhttp.NewChatHandler(srv.ChatService(), requestResolver)
	wsHandler := webhttp.NewWSHandler(
		srv.StreamHub(),
		requestResolver,
		websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	)
	appMux := http.NewServeMux()
	appMux.HandleFunc("/chat", chatHandler)
	appMux.HandleFunc("/chat/", chatHandler)
	appMux.HandleFunc("/ws", wsHandler)
	registerProfileAPIHandlers(appMux, requestResolver)
	timelineLogger := log.With().Str("component", "webchat").Str("route", "/api/timeline").Logger()
	timelineHandler := webhttp.NewTimelineHandler(srv.TimelineService(), timelineLogger)
	appMux.HandleFunc("/api/timeline", timelineHandler)
	appMux.HandleFunc("/api/timeline/", timelineHandler)
	serveAppConfigJS := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = io.WriteString(w, appConfigJS)
	}
	appMux.HandleFunc("/app-config.js", serveAppConfigJS)
	appMux.Handle("/api/", srv.APIHandler())
	appMux.Handle("/", srv.UIHandler())

	// HTTP server and run, with optional root mounting
	httpSrv := srv.HTTPServer()
	if httpSrv == nil {
		return errors.New("http server is not initialized")
	}

	// If --root is not "/", mount router under that root with a parent mux
	if s.Root != "" && s.Root != "/" {
		parent := http.NewServeMux()
		// Normalize prefix: ensure it starts with "/" and ends with "/"
		prefix := s.Root
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		if !strings.HasSuffix(prefix, "/") {
			prefix = prefix + "/"
		}
		parent.HandleFunc("/app-config.js", serveAppConfigJS)
		parent.Handle(prefix, http.StripPrefix(strings.TrimRight(prefix, "/"), appMux))
		httpSrv.Handler = parent
		log.Info().Str("root", prefix).Msg("mounted webchat under custom root")
	} else {
		httpSrv.Handler = appMux
	}

	return srv.Run(ctx)
}

func main() {
	root := &cobra.Command{Use: "web-chat", PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logging.InitLoggerFromCobra(cmd)
	}}

	helpSystem := help.NewHelpSystem()
	help_cmd.SetupCobraRootCommand(helpSystem, root)

	if err := clay.InitGlazed("pinocchio", root); err != nil {
		cobra.CheckErr(err)
	}

	timelinecmd.AddToRootCommand(root)

	c, err := NewCommand()
	cobra.CheckErr(err)
	command, err := cli.BuildCobraCommand(c, cli.WithCobraMiddlewaresFunc(geppettosections.GetCobraCommandGeppettoMiddlewares))
	cobra.CheckErr(err)
	root.AddCommand(command)
	cobra.CheckErr(root.Execute())
}
