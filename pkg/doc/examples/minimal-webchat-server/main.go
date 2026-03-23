package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	clay "github.com/go-go-golems/clay/pkg"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	cmdsources "github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const systemPrompt = `You are a concise demo assistant for a minimal Pinocchio webchat app.`

type Command struct {
	*cmds.CommandDescription
}

func pinocchioMiddlewares(parsed *values.Values, cmd *cobra.Command, args []string) ([]cmdsources.Middleware, error) {
	configFiles, err := profilebootstrap.ResolveCLIConfigFiles(parsed)
	if err != nil {
		return nil, errors.Wrap(err, "resolve config files")
	}

	return []cmdsources.Middleware{
		cmdsources.FromCobra(cmd, fields.WithSource("cobra")),
		cmdsources.FromArgs(args, fields.WithSource("arguments")),
		cmdsources.FromEnv("PINOCCHIO", fields.WithSource("env")),
		cmdsources.FromFiles(
			configFiles,
			cmdsources.WithConfigFileMapper(profilebootstrap.MapPinocchioConfigFile),
			cmdsources.WithParseOptions(fields.WithSource("config")),
		),
		cmdsources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	}, nil
}

func NewCommand() (*Command, error) {
	profileSection, err := profilebootstrap.NewProfileSettingsSection()
	if err != nil {
		return nil, errors.Wrap(err, "create profile settings section")
	}
	baseSections, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		return nil, errors.Wrap(err, "create geppetto base sections")
	}

	desc := cmds.NewCommandDescription(
		"minimal-webchat",
		cmds.WithShort("Start a minimal Pinocchio webchat server"),
		cmds.WithFlags(
			fields.New("addr", fields.TypeString, fields.WithDefault(":8080"), fields.WithHelp("HTTP listen address")),
		),
		cmds.WithSections(append([]schema.Section{profileSection}, baseSections...)...),
	)
	return &Command{CommandDescription: desc}, nil
}

func (c *Command) RunIntoWriter(ctx context.Context, parsed *values.Values, _ io.Writer) error {
	type serverSettings struct {
		Addr string `glazed:"addr"`
	}
	settings := &serverSettings{}
	if err := parsed.DecodeSectionInto(values.DefaultSlug, settings); err != nil {
		return errors.Wrap(err, "decode server settings")
	}

	profileSelection, err := profilebootstrap.ResolveCLIProfileSelection(parsed)
	if err != nil {
		return errors.Wrap(err, "resolve profile selection")
	}

	var (
		profileRegistry     gepprofiles.Registry
		defaultRegistrySlug gepprofiles.RegistrySlug
		registryCleanup     func()
	)
	if len(profileSelection.ProfileRegistries) > 0 {
		specs, err := gepprofiles.ParseRegistrySourceSpecs(profileSelection.ProfileRegistries)
		if err != nil {
			return errors.Wrap(err, "parse profile registry specs")
		}
		chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(ctx, specs)
		if err != nil {
			return errors.Wrap(err, "initialize profile registry")
		}
		profileRegistry = chain
		defaultRegistrySlug = chain.DefaultRegistrySlug()
		registryCleanup = func() { _ = chain.Close() }
	}
	if registryCleanup != nil {
		defer registryCleanup()
	}

	baseInferenceSettings, _, err := profilebootstrap.ResolveBaseInferenceSettings(parsed)
	if err != nil {
		return errors.Wrap(err, "resolve base inference settings")
	}

	runtimeBuilder := newRuntimeBuilder(baseInferenceSettings)
	requestResolver := newRequestResolver(profileRegistry, defaultRegistrySlug, baseInferenceSettings)

	srv, err := webchat.NewServer(
		ctx,
		parsed,
		nil,
		webchat.WithRuntimeComposer(runtimeBuilder),
	)
	if err != nil {
		return errors.Wrap(err, "new webchat server")
	}

	chatHandler := webhttp.NewChatHandler(srv.ChatService(), requestResolver)
	wsHandler := webhttp.NewWSHandler(
		srv.StreamHub(),
		requestResolver,
		websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	)
	timelineHandler := webhttp.NewTimelineHandler(
		srv.TimelineService(),
		log.With().Str("component", "webchat").Logger(),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", chatHandler)
	mux.HandleFunc("/chat/", chatHandler)
	mux.HandleFunc("/ws", wsHandler)
	mux.HandleFunc("/api/timeline", timelineHandler)
	mux.HandleFunc("/api/timeline/", timelineHandler)
	mux.Handle("/api/", srv.APIHandler())
	mux.Handle("/", srv.UIHandler())

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	httpServer := &http.Server{Addr: settings.Addr, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = httpServer.Close()
	}()

	log.Info().Str("addr", settings.Addr).Msg("starting minimal webchat server")
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func main() {
	_ = logging.InitEarlyLoggingFromArgs(os.Args[1:], "pinocchio-minimal-webchat")

	rootCmd := &cobra.Command{
		Use:   "pinocchio-minimal-webchat",
		Short: "Minimal Pinocchio webchat example",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return logging.InitLoggerFromCobra(cmd)
		},
	}

	helpSystem := help.NewHelpSystem()
	help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

	if err := clay.InitGlazed("pinocchio-minimal-webchat", rootCmd); err != nil {
		cobra.CheckErr(err)
	}

	cmd, err := NewCommand()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create command")
	}

	cobraCmd, err := cli.BuildCobraCommand(
		cmd,
		cli.WithCobraMiddlewaresFunc(pinocchioMiddlewares),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build cobra command")
	}

	rootCmd.AddCommand(cobraCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRuntimeBuilder(baseSettings *aisettings.InferenceSettings) infruntime.RuntimeBuilder {
	return infruntime.RuntimeBuilderFunc(func(ctx context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		resolved := req.ResolvedInferenceSettings
		if resolved == nil {
			resolved = baseSettings
		}

		eng, err := infruntime.BuildEngineFromSettingsWithMiddlewares(ctx, resolved, systemPrompt, nil)
		if err != nil {
			return infruntime.ComposedRuntime{}, errors.Wrap(err, "build engine")
		}

		return infruntime.ComposedRuntime{
			Engine:           eng,
			SeedSystemPrompt: systemPrompt,
			RuntimeKey:       req.ProfileKey,
		}, nil
	})
}

type requestResolver struct {
	profileRegistry     gepprofiles.Registry
	defaultRegistrySlug gepprofiles.RegistrySlug
	baseSettings        *aisettings.InferenceSettings
}

func newRequestResolver(
	registry gepprofiles.Registry,
	defaultSlug gepprofiles.RegistrySlug,
	baseSettings *aisettings.InferenceSettings,
) webhttp.ConversationRequestResolver {
	return &requestResolver{
		profileRegistry:     registry,
		defaultRegistrySlug: defaultSlug,
		baseSettings:        baseSettings,
	}
}

func (r *requestResolver) Resolve(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
	var body webhttp.ChatRequestBody
	if req.Method == http.MethodPost && req.Body != nil {
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{
				Status:    http.StatusBadRequest,
				ClientMsg: "invalid request body",
				Err:       err,
			}
		}
	}

	convID := body.ConvID
	if convID == "" {
		convID = req.URL.Query().Get("conv_id")
	}

	profileKey := body.Profile
	if profileKey == "" {
		profileKey = "default"
	}

	var resolvedSettings *aisettings.InferenceSettings
	if r.profileRegistry != nil {
		resolvedProfile, err := r.profileRegistry.ResolveEngineProfile(
			req.Context(),
			gepprofiles.ResolveInput{
				EngineProfileSlug: gepprofiles.EngineProfileSlug(profileKey),
				RegistrySlug:      r.defaultRegistrySlug,
			},
		)
		if err == nil && resolvedProfile != nil && resolvedProfile.InferenceSettings != nil {
			merged, mergeErr := gepprofiles.MergeInferenceSettings(r.baseSettings, resolvedProfile.InferenceSettings)
			if mergeErr == nil {
				resolvedSettings = merged
			}
		}
	}
	if resolvedSettings == nil {
		resolvedSettings = r.baseSettings
	}

	return webhttp.ResolvedConversationRequest{
		ConvID:                    convID,
		RuntimeKey:                profileKey,
		ResolvedInferenceSettings: resolvedSettings,
		Prompt:                    body.Prompt,
		IdempotencyKey:            webhttp.IdempotencyKeyFromRequest(req, &body),
	}, nil
}
