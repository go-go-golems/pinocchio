package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	cmdsources "github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	aiconfig "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
	thinkingmode "github.com/go-go-golems/pinocchio/cmd/web-chat/thinkingmode"
	timelinecmd "github.com/go-go-golems/pinocchio/cmd/web-chat/timeline"
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

const webChatProfileSettingsSectionSlug = "profile-settings"
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

func resolveProfileRegistries(parsed *values.Values, defaultSectionValue string) string {
	resolved, _ := resolveProfileRegistriesWithSource(parsed, defaultSectionValue)
	return resolved
}

func resolveProfileRegistriesWithSource(parsed *values.Values, defaultSectionValue string) (string, string) {
	resolved := strings.TrimSpace(defaultSectionValue)
	if resolved != "" {
		return resolved, "default-section"
	}

	if parsed != nil {
		profileSettings := struct {
			ProfileRegistries string `glazed:"profile-registries"`
		}{}
		if err := parsed.DecodeSectionInto(webChatProfileSettingsSectionSlug, &profileSettings); err == nil {
			resolved = strings.TrimSpace(profileSettings.ProfileRegistries)
			if resolved != "" {
				return resolved, webChatProfileSettingsSectionSlug
			}
		}
	}

	resolved = defaultPinocchioProfileRegistriesIfPresent()
	if resolved != "" {
		return resolved, "xdg-default"
	}
	return "", ""
}

func defaultPinocchioProfileRegistriesIfPresent() string {
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		return ""
	}
	path := filepath.Join(configDir, "pinocchio", "profiles.yaml")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

func defaultPinocchioConfigFileIfPresent() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return ""
	}
	path := filepath.Join(homeDir, ".pinocchio", "config.yaml")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

func webChatConfigFileMapper(rawConfig interface{}) (map[string]map[string]interface{}, error) {
	configMap, ok := rawConfig.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("expected map[string]interface{}, got %T", rawConfig)
	}

	result := make(map[string]map[string]interface{})
	excludedKeys := map[string]bool{
		"repositories": true,
	}
	for key, value := range configMap {
		if excludedKeys[key] {
			continue
		}
		layerParams, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		result[key] = layerParams
	}
	return result, nil
}

func resolveWebChatConfigFiles(parsed *values.Values) []string {
	files := make([]string, 0, 2)
	if defaultFile := defaultPinocchioConfigFileIfPresent(); defaultFile != "" {
		files = append(files, defaultFile)
	}
	if parsed != nil {
		commandSettings := &cli.CommandSettings{}
		if err := parsed.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings); err == nil {
			explicit := strings.TrimSpace(commandSettings.ConfigFile)
			if explicit != "" {
				duplicate := false
				for _, f := range files {
					if f == explicit {
						duplicate = true
						break
					}
				}
				if !duplicate {
					files = append(files, explicit)
				}
			}
		}
	}
	return files
}

func resolveWebChatBaseStepSettings(parsed *values.Values) (*aiconfig.StepSettings, []string, error) {
	sections_, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		return nil, nil, errors.Wrap(err, "create hidden geppetto sections")
	}
	schema_ := schema.NewSchema(schema.WithSections(sections_...))
	parsedValues := values.New()
	configFiles := resolveWebChatConfigFiles(parsed)
	if err := cmdsources.Execute(
		schema_,
		parsedValues,
		cmdsources.FromEnv("PINOCCHIO", fields.WithSource("env")),
		cmdsources.FromFiles(
			configFiles,
			cmdsources.WithConfigFileMapper(webChatConfigFileMapper),
			cmdsources.WithParseOptions(fields.WithSource("config")),
		),
		cmdsources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	); err != nil {
		return nil, configFiles, errors.Wrap(err, "resolve hidden web-chat base step settings")
	}
	stepSettings, err := aiconfig.NewStepSettingsFromParsedValues(parsedValues)
	if err != nil {
		return nil, configFiles, errors.Wrap(err, "build step settings from hidden parsed values")
	}
	return stepSettings, configFiles, nil
}

func newWebChatProfileSettingsSection() (schema.Section, error) {
	return schema.NewSection(
		webChatProfileSettingsSectionSlug,
		"Profile settings",
		schema.WithFields(
			fields.New(
				"profile",
				fields.TypeString,
				fields.WithHelp("Load the profile"),
			),
			fields.New(
				"profile-registries",
				fields.TypeString,
				fields.WithHelp("Comma-separated profile registry sources (yaml/sqlite/sqlite-dsn)"),
			),
		),
	)
}

func NewCommand() (*Command, error) {
	profileSettingsSection, err := newWebChatProfileSettingsSection()
	if err != nil {
		return nil, errors.Wrap(err, "create web-chat profile settings section")
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
			fields.New("timeline-js-script", fields.TypeStringList, fields.WithHelp("Path to JavaScript SEM reducer/handler script (repeat flag or pass comma-separated list)")),
			fields.New("turns-dsn", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DSN for durable turn snapshots (enables GET /turns); preferred over turns-db")),
			fields.New("turns-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DB file path for durable turn snapshots (enables GET /turns); DSN is derived with WAL/busy_timeout")),
		),
		cmds.WithSections(profileSettingsSection, redisLayer),
	)
	return &Command{CommandDescription: desc}, nil
}

func (c *Command) RunIntoWriter(ctx context.Context, parsed *values.Values, _ io.Writer) error {
	type serverSettings struct {
		Root              string   `glazed:"root"`
		DebugAPI          bool     `glazed:"debug-api"`
		ProfileRegistries string   `glazed:"profile-registries"`
		TimelineJSScripts []string `glazed:"timeline-js-script"`
	}
	s := &serverSettings{}
	if err := parsed.DecodeSectionInto(values.DefaultSlug, s); err != nil {
		return errors.Wrap(err, "decode server settings")
	}
	var profileRegistriesSource string
	s.ProfileRegistries, profileRegistriesSource = resolveProfileRegistriesWithSource(parsed, s.ProfileRegistries)
	if strings.TrimSpace(s.ProfileRegistries) != "" {
		log.Info().
			Str("source", profileRegistriesSource).
			Str("profile_registries", s.ProfileRegistries).
			Msg("resolved profile registry sources")
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

	profileRegistryEntries, err := gepprofiles.ParseProfileRegistrySourceEntries(s.ProfileRegistries)
	if err != nil {
		return errors.Wrap(err, "parse profile registry sources")
	}
	profileRegistrySpecs, err := gepprofiles.ParseRegistrySourceSpecs(profileRegistryEntries)
	if err != nil {
		return errors.Wrap(err, "parse profile registry source specs")
	}
	profileRegistryChain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(ctx, profileRegistrySpecs)
	if err != nil {
		return errors.Wrap(err, "initialize profile registry")
	}
	defer func() {
		_ = profileRegistryChain.Close()
	}()
	var profileRegistry gepprofiles.Registry = profileRegistryChain

	middlewareRegistry, err := newWebChatMiddlewareDefinitionRegistry()
	if err != nil {
		return errors.Wrap(err, "create middleware definition registry")
	}
	baseStepSettings, configFiles, err := resolveWebChatBaseStepSettings(parsed)
	if err != nil {
		return err
	}
	log.Debug().
		Strs("config_files", configFiles).
		Interface("step_metadata", baseStepSettings.GetMetadata()).
		Msg("resolved hidden web-chat base step settings")
	runtimeComposer := newProfileRuntimeComposer(middlewareRegistry, middlewarecfg.BuildDeps{
		Values: map[string]any{
			dependencyAgentModeServiceKey: amSvc,
			dependencySQLiteDBKey:         dbWithRegexp,
		},
	}, baseStepSettings)
	requestResolver := newProfileRequestResolver(profileRegistry, profileRegistryChain.DefaultRegistrySlug(), baseStepSettings)

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
		AppName: webChatCLIAppName,
		ConfigFilesFunc: func(_ *values.Values, _ *cobra.Command, _ []string) ([]string, error) {
			// Hidden base-settings parsing owns config-file loading so we can
			// reuse pinocchio config conventions without exposing AI flags.
			return nil, nil
		},
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
