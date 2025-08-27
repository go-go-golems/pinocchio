package main

import (
	"context"
	"database/sql"
	"embed"
	"io"
	"net/http"

	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	geppettolayers "github.com/go-go-golems/geppetto/pkg/layers"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	geppettomw "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	sqlitetool "github.com/go-go-golems/pinocchio/pkg/middlewares/sqlitetool"
	toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
	"github.com/rs/zerolog/log"
)

//go:embed static
var staticFS embed.FS

// no package-level root; we will build a cobra command dynamically in main()

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
			parameters.NewParameterDefinition("enable-agentmode", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Enable agent mode middleware")),
			parameters.NewParameterDefinition("idle-timeout-seconds", parameters.ParameterTypeInteger, parameters.WithDefault(60), parameters.WithHelp("Stop per-conversation reader after N seconds with no sockets (0=disabled)")),
		),
		cmds.WithLayersList(append(geLayers, redisLayer)...),
	)
	return &Command{CommandDescription: desc}, nil
}

func (c *Command) RunIntoWriter(ctx context.Context, parsed *layers.ParsedLayers, _ io.Writer) error {
    // Build webchat router and register middlewares/tools/profiles
    r, err := webchat.NewRouter(ctx, parsed, staticFS)
    if err != nil {
        return errors.Wrap(err, "new webchat router")
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

    // Register middlewares
    r.RegisterMiddleware("agentmode", func(cfg any) geppettomw.Middleware { return agentmode.NewMiddleware(amSvc, cfg.(agentmode.Config)) })
    r.RegisterMiddleware("sqlite", func(cfg any) geppettomw.Middleware {
        c := sqlitetool.Config{DB: dbWithRegexp}
        if cfg_, ok := cfg.(sqlitetool.Config); ok { c = cfg_ }
        return sqlitetool.NewMiddleware(c)
    })

    // Register calculator tool
    r.RegisterTool("calculator", func(reg geptools.ToolRegistry) error {
        if im, ok := reg.(*geptools.InMemoryToolRegistry); ok {
            return toolspkg.RegisterCalculatorTool(im)
        }
        im2 := geptools.NewInMemoryToolRegistry()
        if err := toolspkg.RegisterCalculatorTool(im2); err != nil { return err }
        for _, td := range im2.ListTools() { _ = reg.RegisterTool(td.Name, td) }
        return nil
    })

    // Profiles
    r.AddProfile(&webchat.Profile{Slug: "default", DefaultPrompt: "You are a helpful assistant. Be concise.", DefaultMws: []webchat.MiddlewareUse{}})
    r.AddProfile(&webchat.Profile{Slug: "agent", DefaultPrompt: "You are a helpful assistant. Be concise.", DefaultMws: []webchat.MiddlewareUse{{Name: "agentmode", Config: amCfg}}})

    // Lightweight helper endpoints to switch profile from the UI via fetch GET
    // GET /default → sets a cookie chat_profile=default and 204s
    // GET /agent   → sets a cookie chat_profile=agent and 204s
    r.HandleFunc("/default", func(w http.ResponseWriter, r *http.Request) {
        http.SetCookie(w, &http.Cookie{Name: "chat_profile", Value: "default", Path: "/", SameSite: http.SameSiteLaxMode})
        log.Info().Str("component", "profile-switch").Str("profile", "default").Str("remote", r.RemoteAddr).Msg("set chat_profile cookie")
        w.WriteHeader(http.StatusNoContent)
    })
    r.HandleFunc("/agent", func(w http.ResponseWriter, r *http.Request) {
        http.SetCookie(w, &http.Cookie{Name: "chat_profile", Value: "agent", Path: "/", SameSite: http.SameSiteLaxMode})
        log.Info().Str("component", "profile-switch").Str("profile", "agent").Str("remote", r.RemoteAddr).Msg("set chat_profile cookie")
        w.WriteHeader(http.StatusNoContent)
    })

    // HTTP server and run
    httpSrv, err := r.BuildHTTPServer()
    if err != nil { return errors.Wrap(err, "build http server") }
    srv := webchat.NewFromRouter(ctx, r, httpSrv)
    return srv.Run(ctx)
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
