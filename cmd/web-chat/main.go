package main

import (
	"context"
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

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
	appserver "github.com/go-go-golems/pinocchio/cmd/web-chat/app"
	thinkingmode "github.com/go-go-golems/pinocchio/cmd/web-chat/thinkingmode"
	timelinecmd "github.com/go-go-golems/pinocchio/cmd/web-chat/timeline"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
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

const webChatCLIAppName = "pinocchio"

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
	profileSettingsSection, err := profilebootstrap.NewProfileSettingsSection()
	if err != nil {
		return nil, errors.Wrap(err, "create web-chat profile settings section")
	}
	clientSection, err := aisettings.NewClientValueSection()
	if err != nil {
		return nil, errors.Wrap(err, "create web-chat ai-client section")
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
			fields.New("idle-timeout-seconds", fields.TypeInteger, fields.WithDefault(60), fields.WithHelp("Stop per-conversation reader after N seconds with no sockets (0=disabled)")),
			fields.New("evict-idle-seconds", fields.TypeInteger, fields.WithDefault(300), fields.WithHelp("Evict conversations after N seconds idle (0=disabled)")),
			fields.New("evict-interval-seconds", fields.TypeInteger, fields.WithDefault(60), fields.WithHelp("Sweep idle conversations every N seconds (0=disabled)")),
			fields.New("root", fields.TypeString, fields.WithDefault("/"), fields.WithHelp("Serve the chat UI under a given URL root (e.g., /chat)")),
			fields.New("debug-api", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Enable debug API endpoints under /api/debug/*")),
			fields.New("timeline-dsn", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DSN for durable timeline snapshots (enables GET /timeline); preferred over timeline-db")),
			fields.New("timeline-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DB file path for durable timeline snapshots (enables GET /timeline); DSN is derived with WAL/busy_timeout")),
			fields.New("timeline-js-script", fields.TypeStringList, fields.WithHelp("Path to JavaScript SEM reducer/handler script (repeat flag or pass comma-separated list)")),
			fields.New("turns-dsn", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DSN for durable turn snapshots (enables GET /turns); preferred over turns-db")),
			fields.New("turns-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DB file path for durable turn snapshots (enables GET /turns); DSN is derived with WAL/busy_timeout")),
		),
		cmds.WithSections(profileSettingsSection, clientSection, redisLayer),
	)
	return &Command{CommandDescription: desc}, nil
}

func (c *Command) RunIntoWriter(ctx context.Context, parsed *values.Values, _ io.Writer) error {
	type serverSettings struct {
		Root              string   `glazed:"root"`
		DebugAPI          bool     `glazed:"debug-api"`
		TimelineJSScripts []string `glazed:"timeline-js-script"`
		TimelineDSN       string   `glazed:"timeline-dsn"`
		TimelineDB        string   `glazed:"timeline-db"`
	}
	s := &serverSettings{}
	if err := parsed.DecodeSectionInto(values.DefaultSlug, s); err != nil {
		return errors.Wrap(err, "decode server settings")
	}
	profileRuntime, err := profilebootstrap.ResolveCLIProfileRuntime(ctx, parsed)
	if err != nil {
		return errors.Wrap(err, "resolve profile runtime")
	}
	if profileRuntime != nil && profileRuntime.Close != nil {
		defer profileRuntime.Close()
	}
	profileSelection := profileRuntime.ProfileSettings
	if len(profileSelection.ProfileRegistries) > 0 {
		log.Info().
			Strs("profile_registries", profileSelection.ProfileRegistries).
			Msg("resolved profile registry sources")
	}

	appConfigJS, err := runtimeConfigScript(s.Root, s.DebugAPI)
	if err != nil {
		return errors.Wrap(err, "build runtime config script")
	}

	// Agent mode configuration (optional)
	amSvc := agentmode.NewStaticService([]*agentmode.AgentMode{
		{Name: "financial_analyst", Prompt: "You are a financial transaction analyst. Analyze transactions and propose categories."},
		{Name: "category_regexp_designer", Prompt: "Design regex patterns to categorize transactions. Verify with SQL counts before proposing changes."},
		{Name: "category_regexp_reviewer", Prompt: "Review proposed regex patterns and assess over/under matching risks."},
	})

	var (
		profileRegistry     gepprofiles.Registry
		defaultRegistrySlug gepprofiles.RegistrySlug
	)
	registryChain := profileRuntime.ProfileRegistryChain
	if registryChain != nil {
		profileRegistry = registryChain.Registry
		defaultRegistrySlug = registryChain.DefaultRegistrySlug
	}

	middlewareRegistry, err := newWebChatMiddlewareDefinitionRegistry()
	if err != nil {
		return errors.Wrap(err, "create middleware definition registry")
	}
	hiddenBaseInferenceSettings, configFiles, err := profilebootstrap.ResolveBaseInferenceSettings(parsed)
	if err != nil {
		return err
	}
	baseInferenceSettings, err := profilebootstrap.ResolveParsedBaseInferenceSettingsWithBase(parsed, hiddenBaseInferenceSettings)
	if err != nil {
		return errors.Wrap(err, "resolve web-chat base inference settings from hidden base and parsed values")
	}
	log.Debug().
		Strs("config_files", configFiles).
		Interface("step_metadata", baseInferenceSettings.GetMetadata()).
		Msg("resolved web-chat base inference settings")
	runtimeComposer := newProfileRuntimeComposer(middlewareRegistry, middlewarecfg.BuildDeps{
		Values: map[string]any{
			dependencyAgentModeServiceKey: amSvc,
		},
	}, baseInferenceSettings)
	requestResolver := newProfileRequestResolver(profileRegistry, defaultRegistrySlug, baseInferenceSettings)
	canonicalApp, err := appserver.NewServer(
		appserver.WithSQLiteDSN(s.TimelineDSN),
		appserver.WithSQLiteDBPath(s.TimelineDB),
	)
	if err != nil {
		return errors.Wrap(err, "build canonical evtstream-backed app")
	}
	defer func() {
		_ = canonicalApp.Close()
	}()

	if err := configureTimelineJSScripts(s.TimelineJSScripts); err != nil {
		return err
	}

	// Register app-owned thinking-mode SEM/timeline handlers.
	thinkingmode.Register()

	// Build webchat server and register middlewares/tools/profile handlers.
	srv, err := webchat.NewServer(
		ctx,
		parsed,
		staticFS,
		webchat.WithRuntimeComposer(runtimeComposer),
		webchat.WithEventSinkWrapper(newAgentModeStructuredSinkWrapper()),
		webchat.WithDebugRoutesEnabled(s.DebugAPI),
	)
	if err != nil {
		return errors.Wrap(err, "new webchat server")
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
	appMux.HandleFunc("/api/chat/sessions", canonicalApp.HandleCreateSession)
	appMux.HandleFunc("/api/chat/sessions/", canonicalApp.HandleSessionRoutes)
	appMux.HandleFunc("/api/chat/ws", canonicalApp.HandleWS)
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
	command, err := cli.BuildCobraCommand(c, cli.WithParserConfig(cli.CobraParserConfig{
		// Hidden base-settings parsing owns config-file loading so we can
		// reuse pinocchio config conventions without exposing AI flags.
		AppName: webChatCLIAppName,
	}))
	cobra.CheckErr(err)
	for _, name := range []string{"print-yaml", "print-parsed-fields", "print-schema"} {
		if flag := command.Flags().Lookup(name); flag != nil {
			flag.Hidden = true
		}
	}
	root.AddCommand(command)
	cobra.CheckErr(root.Execute())
}
