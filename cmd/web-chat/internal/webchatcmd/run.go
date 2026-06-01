package webchatcmd

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/appserver"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/middlewaredefs"
	agentmodeplugin "github.com/go-go-golems/pinocchio/cmd/web-chat/internal/plugins/agentmode"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/profiles"
	webchatruntime "github.com/go-go-golems/pinocchio/cmd/web-chat/internal/runtime"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/webapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/plugins"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/widgets"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	"github.com/pkg/errors"
	zlog "github.com/rs/zerolog/log"
)

type ServerSettings struct {
	Addr        string `glazed:"addr"`
	Root        string `glazed:"root"`
	TimelineDSN string `glazed:"timeline-dsn"`
	TimelineDB  string `glazed:"timeline-db"`
	TurnsDSN    string `glazed:"turns-dsn"`
	TurnsDB     string `glazed:"turns-db"`
}

func Run(ctx context.Context, parsed *values.Values, staticFS fs.FS) error {
	s := &ServerSettings{}
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
		zlog.Info().Strs("profile_registries", profileSelection.ProfileRegistries).Msg("resolved profile registry sources")
	}

	appConfigJS, err := webapp.RuntimeConfigScript(s.Root)
	if err != nil {
		return errors.Wrap(err, "build runtime config script")
	}

	amSvc := agentmode.NewStaticService([]*agentmode.AgentMode{
		{Name: "financial_analyst", Prompt: "You are a financial transaction analyst. Analyze transactions and propose categories."},
		{Name: "category_regexp_designer", Prompt: "Design regex patterns to categorize transactions. Verify with SQL counts before proposing changes."},
		{Name: "category_regexp_reviewer", Prompt: "Review proposed regex patterns and assess over/under matching risks."},
	})

	middlewareRegistry, err := middlewaredefs.NewRegistry()
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
	turnStore, closeTurnStore, err := serverkit.OpenTurnStore(serverkit.StoreOptions{TurnsDSN: s.TurnsDSN, TurnsDB: s.TurnsDB})
	if err != nil {
		return err
	}
	defer func() { _ = closeTurnStore() }()

	runtimeComposer := webchatruntime.NewProfileRuntimeComposer(middlewareRegistry, middlewarecfg.BuildDeps{
		Values: map[string]any{
			middlewaredefs.DependencyAgentModeServiceKey: amSvc,
		},
	}, baseInferenceSettings).WithTurnStore(turnStore)

	var (
		profileRegistry     gepprofiles.Registry
		defaultRegistrySlug gepprofiles.RegistrySlug
	)
	if profileRuntime != nil && profileRuntime.ProfileRegistryChain != nil {
		profileRegistry = profileRuntime.ProfileRegistryChain.Registry
		defaultRegistrySlug = profileRuntime.ProfileRegistryChain.DefaultRegistrySlug
	}
	requestResolver := profiles.NewRequestResolver(profileRegistry, defaultRegistrySlug, baseInferenceSettings)
	canonicalRuntimeResolver := webchatruntime.NewCanonicalRuntimeResolver(requestResolver, runtimeComposer)

	frontendToolManager := frontendtools.NewManager()
	canonicalApp, err := appserver.NewServer(
		appserver.WithSQLiteDSN(s.TimelineDSN),
		appserver.WithSQLiteDBPath(s.TimelineDB),
		appserver.WithRuntimeResolver(canonicalRuntimeResolver),
		appserver.WithTurnStore(turnStore),
		appserver.WithTurnsDBPath(s.TurnsDB),
		appserver.WithFrontendToolManager(frontendToolManager),
		appserver.WithChatPlugins(agentmodeplugin.NewPlugin(), plugins.NewReasoningPlugin(), plugins.NewToolCallPlugin(), frontendtools.NewPlugin(), widgets.NewWidgetPlugin()),
	)
	if err != nil {
		return errors.Wrap(err, "build canonical evtstream-backed app")
	}

	appMux := webapp.NewMux(webapp.MuxOptions{
		StaticFS:              staticFS,
		AppConfigJS:           appConfigJS,
		RequestResolver:       requestResolver,
		ChatServer:            canonicalApp,
		MiddlewareDefinitions: middlewareRegistry,
		ExtensionSchemas:      starterSuggestionExtensionSchemas(),
	})
	handler := webapp.MountRoot(s.Root, appMux, appConfigJS)
	httpSrv := &http.Server{
		Addr:              s.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return webapp.RunHTTPServer(ctx, httpSrv, canonicalApp.Close)
}

func starterSuggestionExtensionSchemas() []profiles.ExtensionSchemaDocument {
	return []profiles.ExtensionSchemaDocument{
		{
			Key: "webchat.starter_suggestions@v1",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
						"default": []any{},
					},
				},
				"required":             []any{"items"},
				"additionalProperties": false,
			},
		},
	}
}
