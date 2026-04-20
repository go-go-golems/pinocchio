package main

import (
	"context"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	appserver "github.com/go-go-golems/pinocchio/cmd/web-chat/app"
	timelinecmd "github.com/go-go-golems/pinocchio/cmd/web-chat/timeline"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
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

func buildAppConfigHandler(appConfigJS string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
}

func fsSub(staticFS fs.FS, path string) (fs.FS, error) {
	return fs.Sub(staticFS, path)
}

func registerStaticUIHandlers(mux *http.ServeMux, staticFS fs.FS) {
	if mux == nil || staticFS == nil {
		return
	}
	logger := log.With().Str("component", "web-chat").Logger()
	if staticSub, err := fsSub(staticFS, "static"); err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	} else {
		logger.Warn().Err(err).Msg("failed to mount /static/ asset handler")
	}
	if distAssets, err := fsSub(staticFS, "static/dist/assets"); err == nil {
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(distAssets))))
	} else {
		logger.Warn().Err(err).Msg("no built dist assets found under static/dist/assets")
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			http.NotFound(w, r)
			return
		}
		if b, err := fs.ReadFile(staticFS, "static/dist/index.html"); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		if b, err := fs.ReadFile(staticFS, "static/index.html"); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		http.Error(w, "index not found", http.StatusInternalServerError)
	})
}

func buildAppMux(staticFS fs.FS, appConfigJS string, requestResolver *ProfileRequestResolver, canonicalApp *appserver.Server) *http.ServeMux {
	mux := http.NewServeMux()
	registerProfileAPIHandlers(mux, requestResolver)
	if canonicalApp != nil {
		mux.HandleFunc("/api/chat/sessions", canonicalApp.HandleCreateSession)
		mux.HandleFunc("/api/chat/sessions/", canonicalApp.HandleSessionRoutes)
		mux.HandleFunc("/api/chat/ws", canonicalApp.HandleWS)
	}
	mux.HandleFunc("/app-config.js", buildAppConfigHandler(appConfigJS))
	registerStaticUIHandlers(mux, staticFS)
	return mux
}

func buildRootHandler(root string, appMux http.Handler, appConfigJS string) http.Handler {
	if appMux == nil {
		return http.NotFoundHandler()
	}
	if root == "" || root == "/" {
		return appMux
	}
	prefix := root
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	parent := http.NewServeMux()
	parent.HandleFunc("/app-config.js", buildAppConfigHandler(appConfigJS))
	parent.Handle(prefix, http.StripPrefix(strings.TrimRight(prefix, "/"), appMux))
	log.Info().Str("root", prefix).Msg("mounted webchat under custom root")
	return parent
}

func runHTTPServer(ctx context.Context, srv *http.Server, closeFn func() error) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}
	if srv == nil {
		return errors.New("http server is not initialized")
	}
	srvCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	eg, egCtx := errgroup.WithContext(srvCtx)
	eg.Go(func() error {
		<-egCtx.Done()
		shutdownBase := context.WithoutCancel(ctx)
		shutdownCtx, cancel := context.WithTimeout(shutdownBase, 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		if closeFn != nil {
			return closeFn()
		}
		return nil
	})
	eg.Go(func() error {
		log.Info().Str("addr", srv.Addr).Msg("starting web-chat server")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	return eg.Wait()
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
			fields.New("turns-dsn", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DSN for durable turn snapshots (enables GET /turns); preferred over turns-db")),
			fields.New("turns-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DB file path for durable turn snapshots (enables GET /turns); DSN is derived with WAL/busy_timeout")),
		),
		cmds.WithSections(profileSettingsSection, clientSection, redisLayer),
	)
	return &Command{CommandDescription: desc}, nil
}

func (c *Command) RunIntoWriter(ctx context.Context, parsed *values.Values, _ io.Writer) error {
	type serverSettings struct {
		Addr        string `glazed:"addr"`
		Root        string `glazed:"root"`
		DebugAPI    bool   `glazed:"debug-api"`
		TimelineDSN string `glazed:"timeline-dsn"`
		TimelineDB  string `glazed:"timeline-db"`
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
		log.Info().Strs("profile_registries", profileSelection.ProfileRegistries).Msg("resolved profile registry sources")
	}

	appConfigJS, err := runtimeConfigScript(s.Root, s.DebugAPI)
	if err != nil {
		return errors.Wrap(err, "build runtime config script")
	}

	amSvc := agentmode.NewStaticService([]*agentmode.AgentMode{
		{Name: "financial_analyst", Prompt: "You are a financial transaction analyst. Analyze transactions and propose categories."},
		{Name: "category_regexp_designer", Prompt: "Design regex patterns to categorize transactions. Verify with SQL counts before proposing changes."},
		{Name: "category_regexp_reviewer", Prompt: "Review proposed regex patterns and assess over/under matching risks."},
	})

	middlewareRegistry, err := newWebChatMiddlewareDefinitionRegistry()
	if err != nil {
		return errors.Wrap(err, "create middleware definition registry")
	}
	hiddenBaseInferenceSettings, _, err := profilebootstrap.ResolveBaseInferenceSettings(parsed)
	if err != nil {
		return err
	}
	baseInferenceSettings, err := profilebootstrap.ResolveParsedBaseInferenceSettingsWithBase(parsed, hiddenBaseInferenceSettings)
	if err != nil {
		return errors.Wrap(err, "resolve web-chat base inference settings from hidden base and parsed values")
	}
	runtimeComposer := newProfileRuntimeComposer(middlewareRegistry, middlewarecfg.BuildDeps{
		Values: map[string]any{
			dependencyAgentModeServiceKey: amSvc,
		},
	}, baseInferenceSettings)

	var (
		profileRegistry     gepprofiles.Registry
		defaultRegistrySlug gepprofiles.RegistrySlug
	)
	if profileRuntime != nil && profileRuntime.ProfileRegistryChain != nil {
		profileRegistry = profileRuntime.ProfileRegistryChain.Registry
		defaultRegistrySlug = profileRuntime.ProfileRegistryChain.DefaultRegistrySlug
	}
	requestResolver := newProfileRequestResolver(profileRegistry, defaultRegistrySlug, baseInferenceSettings)
	canonicalRuntimeResolver := newCanonicalRuntimeResolver(requestResolver, runtimeComposer)

	canonicalApp, err := appserver.NewServer(
		appserver.WithSQLiteDSN(s.TimelineDSN),
		appserver.WithSQLiteDBPath(s.TimelineDB),
		appserver.WithRuntimeResolver(canonicalRuntimeResolver),
	)
	if err != nil {
		return errors.Wrap(err, "build canonical evtstream-backed app")
	}

	appMux := buildAppMux(staticFS, appConfigJS, requestResolver, canonicalApp)
	handler := buildRootHandler(s.Root, appMux, appConfigJS)
	httpSrv := &http.Server{
		Addr:              s.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return runHTTPServer(ctx, httpSrv, canonicalApp.Close)
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
