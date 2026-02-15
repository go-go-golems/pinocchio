package main

import (
	"context"
	"database/sql"
	"embed"
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
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
	timelinecmd "github.com/go-go-golems/pinocchio/cmd/web-chat/timeline"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	sqlitetool "github.com/go-go-golems/pinocchio/pkg/middlewares/sqlitetool"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/rs/zerolog/log"
)

//go:embed static
var staticFS embed.FS

// no package-level root; we will build a cobra command dynamically in main()

type Command struct {
	*cmds.CommandDescription
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

	profiles := newChatProfileRegistry(
		"default",
		&chatProfile{Slug: "default", DefaultPrompt: "You are an assistant", DefaultMws: []webchat.MiddlewareUse{}},
		&chatProfile{
			Slug:           "agent",
			DefaultPrompt:  "You are a helpful assistant. Be concise.",
			DefaultMws:     []webchat.MiddlewareUse{{Name: "agentmode", Config: amCfg}},
			AllowOverrides: true,
		},
	)

	middlewareFactories := map[string]webchat.MiddlewareFactory{
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
	runtimeComposer := newWebChatRuntimeComposer(parsed, middlewareFactories)
	requestResolver := newWebChatProfileResolver(profiles)

	// Build webchat router and register middlewares/tools/profile handlers.
	r, err := webchat.NewRouter(
		ctx,
		parsed,
		staticFS,
		webchat.WithRuntimeComposer(runtimeComposer),
	)
	if err != nil {
		return errors.Wrap(err, "new webchat router")
	}

	// Register middlewares
	for name, factory := range middlewareFactories {
		r.RegisterMiddleware(name, factory)
	}

	// Register calculator tool
	r.RegisterTool("calculator", func(reg geptools.ToolRegistry) error {
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

	registerProfileHandlers(r, profiles)
	chatHandler := webchat.NewChatHandler(r.ConversationService(), requestResolver)
	r.HandleFunc("/chat", chatHandler)
	r.HandleFunc("/chat/", chatHandler)

	// HTTP server and run, with optional root mounting
	httpSrv, err := r.BuildHTTPServer()
	if err != nil {
		return errors.Wrap(err, "build http server")
	}

	// If --root is not "/", mount router under that root with a parent mux
	type serverSettings struct {
		Root string `glazed:"root"`
	}
	s := &serverSettings{}
	_ = parsed.DecodeSectionInto(values.DefaultSlug, s)
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
		parent.Handle(prefix, http.StripPrefix(strings.TrimRight(prefix, "/"), r.Handler()))
		httpSrv.Handler = parent
		log.Info().Str("root", prefix).Msg("mounted webchat under custom root")
	}

	srv := webchat.NewFromRouter(ctx, r, httpSrv)
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
