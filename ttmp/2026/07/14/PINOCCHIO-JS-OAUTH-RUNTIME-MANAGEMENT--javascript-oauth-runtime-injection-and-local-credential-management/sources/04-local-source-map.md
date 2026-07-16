# Local implementation evidence

## Geppetto source-injection API
    35	// MiddlewareFactory resolves a named Go middleware from JS options.
    36	type MiddlewareFactory func(options map[string]any) (middleware.Middleware, error)
    37	
    38	// Options configures module behavior for a specific runtime.
    39	type Options struct {
    40		RuntimeOwner              runtimeowner.RuntimeOwner
    41		GoToolRegistry            tools.ToolRegistry
    42		GoMiddlewareFactories     map[string]MiddlewareFactory
    43		EngineProfileRegistry     profiles.RegistryReader
    44		EngineProfileRegistrySpec []string
    45		DefaultInferenceSettings  *aistepssettings.InferenceSettings
    46		// BearerTokenSource supplies JavaScript-created OpenAI-compatible engines
    47		// with a host-owned request-time bearer source. It is intentionally a Go-only
    48		// capability: the module never exports it or credential values to JavaScript.
    49		BearerTokenSource           credentials.BearerTokenSource
    50		UseDefaultProfileResolve    bool
    51		DefaultProfileResolve       profiles.ResolveInput
    52		MiddlewareSchemas           middlewarecfg.DefinitionRegistry
    53		ExtensionCodecs             profiles.ExtensionCodecRegistry
    54		ExtensionSchemas            map[string]map[string]any
    55		DefaultEventSinks           []events.EventSink
    56		EventEmitterManager         *jsevents.Manager
    57		EventEmitterManagerResolver func() (*jsevents.Manager, bool)
    58		DefaultSnapshotHook         toolloop.SnapshotHook
    59		DefaultPersister            enginebuilder.TurnPersister
    60		EnableStorage               bool
    61		DefaultTurnStore            TurnStore
    62		TurnStores                  map[string]TurnStore
    63		Logger                      zerolog.Logger
    64	}
    65	
    66	// NewLoader returns the native geppetto module loader for use with a require
    67	// registry or an xgoja provider wrapper.
    68	func NewLoader(opts Options) require.ModuleLoader {
    69		mod := &module{opts: opts}
    70		return mod.Loader
    71	}
    72	
    73	// Register registers the geppetto native module on a require registry.
    74	func Register(reg *require.Registry, opts Options) {
    75		if reg == nil {
    76			return
    77		}
    78		reg.RegisterNativeModule(ModuleName, NewLoader(opts))
    79	}
    80	
    81	type module struct {
    82		opts Options
    83	}
    84	
    85	type moduleRuntime struct {
    86		vm           *goja.Runtime
    87		runtimeOwner runtimeowner.RuntimeOwner
    88		bridge       *gpruntimebridge.Bridge
    89	
    90		logger zerolog.Logger
    91	
    92		goToolRegistry                  tools.ToolRegistry
    93		goMiddlewareFactories           map[string]MiddlewareFactory
    94		defaultInferenceSettings        *aistepssettings.InferenceSettings
    95		bearerTokenSource               credentials.BearerTokenSource
    96		profileRegistry                 profiles.RegistryReader
    97		profileRegistryCloser           io.Closer
    98		profileRegistrySpec             []string
    99		baseEngineProfileRegistry       profiles.RegistryReader
   100		baseEngineProfileRegistryCloser io.Closer
   101		baseEngineProfileRegistrySpec   []string
   102		useDefaultProfileResolve        bool
   103		defaultProfileResolve           profiles.ResolveInput
   104		middlewareSchemas               middlewarecfg.DefinitionRegistry
   105		extensionCodecs                 profiles.ExtensionCodecRegistry
   106		extensionSchemas                map[string]map[string]any
   107		defaultEventSinks               []events.EventSink
   108		eventEmitterManager             *jsevents.Manager
   109		eventEmitterManagerResolver     func() (*jsevents.Manager, bool)
   110		runtimeLifetimeContext          context.Context
   111		defaultSnapshotHook             toolloop.SnapshotHook
   112		defaultPersister                enginebuilder.TurnPersister
   113		enableStorage                   bool
   114		defaultTurnStore                TurnStore
   115		turnStores                      map[string]TurnStore
   116	}
   117	
   118	func newRuntime(vm *goja.Runtime, opts Options) *moduleRuntime {
   119		lg := opts.Logger
   120		if lg.GetLevel() == zerolog.NoLevel {
   121			lg = zlog.Logger
   122		}
   123		m := &moduleRuntime{
   124			vm:                            vm,
   125			runtimeOwner:                  opts.RuntimeOwner,
   126			logger:                        lg,
   127			goToolRegistry:                opts.GoToolRegistry,
   128			goMiddlewareFactories:         map[string]MiddlewareFactory{},
   129			defaultInferenceSettings:      cloneInferenceSettings(opts.DefaultInferenceSettings),
   130			bearerTokenSource:             opts.BearerTokenSource,
   131			profileRegistry:               opts.EngineProfileRegistry,
   132			profileRegistrySpec:           append([]string(nil), opts.EngineProfileRegistrySpec...),
   133			baseEngineProfileRegistrySpec: append([]string(nil), opts.EngineProfileRegistrySpec...),
   134			useDefaultProfileResolve:      opts.UseDefaultProfileResolve,
   135			defaultProfileResolve:         opts.DefaultProfileResolve,
   136			middlewareSchemas:             opts.MiddlewareSchemas,
   137			extensionCodecs:               opts.ExtensionCodecs,
   138			extensionSchemas:              cloneNestedStringAnyMap(opts.ExtensionSchemas),
   139			defaultEventSinks:             append([]events.EventSink(nil), opts.DefaultEventSinks...),
   140			eventEmitterManager:           opts.EventEmitterManager,
   141			eventEmitterManagerResolver:   opts.EventEmitterManagerResolver,
   142			runtimeLifetimeContext:        context.Background(),
   143			defaultSnapshotHook:           opts.DefaultSnapshotHook,
   144			defaultPersister:              opts.DefaultPersister,
   145			enableStorage:                 opts.EnableStorage,
   146			defaultTurnStore:              opts.DefaultTurnStore,
   147			turnStores:                    map[string]TurnStore{},
   148		}
   149		for name, store := range opts.TurnStores {
   150			if strings.TrimSpace(name) == "" || store == nil {

## Pinocchio JS runtime registration
   180					profilebootstrap.BootstrapConfig(),
   181					parsed,
   182					*debugSettings,
   183					&geppettobootstrap.ResolvedInferenceTrace{
   184						FinalInferenceSettings: runtimeBootstrap.ResolvedEngineSettings.FinalInferenceSettings,
   185						ResolvedEngineProfile:  runtimeBootstrap.ResolvedEngineSettings.ResolvedEngineProfile,
   186					},
   187					geppettobootstrap.InferenceDebugOutputOptions{},
   188				)
   189				return err
   190			}
   191		}
   192		middlewareDefs, buildDeps, err := buildPinocchioJSMiddlewareRegistry()
   193		if err != nil {
   194			return err
   195		}
   196		middlewareFactories := buildPinocchioJSMiddlewareFactories(buildDeps)
   197		turnStore, closeTurnStore, err := openPinocchioJSTurnStore(settings.TurnsDSN, settings.TurnsDB)
   198		if err != nil {
   199			return err
   200		}
   201		defer closeTurnStore()
   202	
   203		scriptDir := filepath.Dir(scriptPath)
   204		rt, err := newPinocchioJSRuntime(ctx, pinocchioJSRuntimeOptions{
   205			ScriptDir:                scriptDir,
   206			DefaultInferenceSettings: runtimeBootstrap.DefaultInferenceSettings,
   207			GoToolRegistry:           goRegistry,
   208			ProfileRegistry:          runtimeBootstrap.ProfileRegistry,
   209			UseDefaultProfileResolve: runtimeBootstrap.UseDefaultProfileResolve,
   210			DefaultProfileResolve:    runtimeBootstrap.DefaultProfileResolve,
   211			GoMiddlewareFactories:    middlewareFactories,
   212			MiddlewareDefinitions:    middlewareDefs,
   213			TurnStore:                turnStore,
   214			Stdout:                   w,
   215			Stderr:                   os.Stderr,
   216		})
   217		if err != nil {
   218			return err
   219		}
   220		defer func() {
   221			_ = rt.Close(context.Background())
   222		}()
   223	
   224		result, err := rt.VM.RunScript(filepath.Base(scriptPath), string(scriptBytes))
   225		if err != nil {
   226			return err
   227		}
   228		if settings.PrintResult && result != nil && !goja.IsUndefined(result) && !goja.IsNull(result) {
   229			b, err := json.MarshalIndent(result.Export(), "", "  ")
   230			if err != nil {
   231				return err
   232			}
   233			fmt.Fprintln(w, string(b))
   234		}
   235		return nil
   236	}
   237	
   238	func resolveJSScriptPath(settings *JSSettings) (string, error) {
   239		flagPath := strings.TrimSpace(settings.ScriptPath)
   240		argPath := strings.TrimSpace(settings.ScriptArg)
   241		if flagPath != "" && argPath != "" {
   242			return "", fmt.Errorf("provide either --script or a positional script path, not both")
   243		}
   244		if flagPath != "" {
   245			return flagPath, nil
   246		}
   247		if argPath != "" {
   248			return argPath, nil
   249		}
   250		return "", fmt.Errorf("--script is required")
   251	}
   252	
   253	type pinocchioJSRuntimeBootstrap struct {
   254		DefaultInferenceSettings *aisettings.InferenceSettings
   255		ResolvedEngineSettings   *profilebootstrap.ResolvedCLIEngineSettings
   256		ProfileRegistry          gepprofiles.RegistryReader
   257		UseDefaultProfileResolve bool
   258		DefaultProfileResolve    gepprofiles.ResolveInput
   259		Close                    func()
   260	}
   261	
   262	func resolvePinocchioJSRuntimeBootstrap(ctx context.Context, parsed *values.Values) (*pinocchioJSRuntimeBootstrap, error) {
   263		resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsed)
   264		if err != nil {
   265			return nil, err
   266		}
   267	
   268		var profileRegistry gepprofiles.RegistryReader
   269		var useDefaultProfileResolve bool
   270		var defaultProfileResolve gepprofiles.ResolveInput
   271		var registryChain *geppettobootstrap.ResolvedProfileRegistryChain
   272		if resolved.ProfileRuntime != nil {
   273			registryChain = resolved.ProfileRuntime.ProfileRegistryChain
   274		}
   275		if registryChain != nil {
   276			profileRegistry = registryChain.Reader
   277			useDefaultProfileResolve = registryChain.DefaultProfileResolve.EngineProfileSlug != ""
   278			defaultProfileResolve = registryChain.DefaultProfileResolve
   279		}
   280	
   281		return &pinocchioJSRuntimeBootstrap{
   282			DefaultInferenceSettings: resolved.FinalInferenceSettings,
   283			ResolvedEngineSettings:   resolved,
   284			ProfileRegistry:          profileRegistry,
   285			UseDefaultProfileResolve: useDefaultProfileResolve,
   286			DefaultProfileResolve:    defaultProfileResolve,
   287			Close:                    resolved.Close,
   288		}, nil
   289	}
   290	
   291	type pinocchioJSRuntimeOptions struct {
   292		ScriptDir                string
   293		DefaultInferenceSettings *aisettings.InferenceSettings
   294		GoToolRegistry           geptools.ToolRegistry
   295		ProfileRegistry          gepprofiles.RegistryReader
   296		UseDefaultProfileResolve bool
   297		DefaultProfileResolve    gepprofiles.ResolveInput
   298		GoMiddlewareFactories    map[string]gp.MiddlewareFactory
   299		MiddlewareDefinitions    middlewarecfg.DefinitionRegistry
   300		TurnStore                gp.TurnStore
   301		Stdout                   io.Writer
   302		Stderr                   io.Writer
   303	}
   304	
   305	func newPinocchioJSRuntime(ctx context.Context, opts pinocchioJSRuntimeOptions) (*gojengine.Runtime, error) {
   306		requireOpts := []require.Option{
   307			require.WithGlobalFolders(
   308				opts.ScriptDir,
   309				filepath.Join(opts.ScriptDir, "node_modules"),
   310			),
   311		}
   312		builder := gojengine.NewRuntimeFactoryBuilder(gojengine.WithRequireOptions(requireOpts...))
   313		factory, err := builder.Build()
   314		if err != nil {
   315			return nil, err
   316		}
   317		rt, err := factory.NewRuntime(
   318			gojengine.WithStartupContext(ctx),
   319			gojengine.WithLifetimeContext(ctx),
   320		)
   321		if err != nil {
   322			return nil, err
   323		}
   324	
   325		reg := require.NewRegistry(requireOpts...)
   326		gpOptions := gp.Options{
   327			RuntimeOwner:             rt.Owner,
   328			GoToolRegistry:           opts.GoToolRegistry,
   329			GoMiddlewareFactories:    opts.GoMiddlewareFactories,
   330			EngineProfileRegistry:    opts.ProfileRegistry,
   331			DefaultInferenceSettings: opts.DefaultInferenceSettings,
   332			UseDefaultProfileResolve: opts.UseDefaultProfileResolve,
   333			DefaultProfileResolve:    opts.DefaultProfileResolve,
   334			MiddlewareSchemas:        opts.MiddlewareDefinitions,
   335		}
   336		if opts.TurnStore != nil {
   337			gpOptions.EnableStorage = true
   338			gpOptions.DefaultTurnStore = opts.TurnStore
   339			gpOptions.DefaultPersister = opts.TurnStore
   340			gpOptions.TurnStores = map[string]gp.TurnStore{"default": opts.TurnStore}
   341		}
   342		gp.Register(reg, gpOptions)
   343		pjs.Register(reg, pjs.Options{
   344			DefaultInferenceSettings: opts.DefaultInferenceSettings,
   345		})
   346		req := reg.Enable(rt.VM)
   347		rt.Require = req
   348	
   349		runtimeCtx := &gojengine.RuntimeInitializationContext{
   350			VM:      rt.VM,

## Pinocchio JS module engine factory
     1	package pinocchio
     2	
     3	import (
     4		"fmt"
     5		"strings"
     6		"time"
     7	
     8		"github.com/dop251/goja"
     9		"github.com/dop251/goja_nodejs/require"
    10		"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    11		aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
    12		aitypes "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
    13	)
    14	
    15	const ModuleName = "pinocchio"
    16	
    17	type Options struct {
    18		DefaultInferenceSettings *aisettings.InferenceSettings
    19	}
    20	
    21	func Register(reg *require.Registry, opts Options) {
    22		if reg == nil {
    23			return
    24		}
    25		reg.RegisterNativeModule(ModuleName, (&module{opts: opts}).Loader)
    26	}
    27	
    28	type module struct {
    29		opts Options
    30	}
    31	
    32	func (m *module) Loader(vm *goja.Runtime, moduleObj *goja.Object) {
    33		exports := moduleObj.Get("exports").(*goja.Object)
    34		enginesObj := vm.NewObject()
    35		if err := enginesObj.Set("fromDefaults", func(call goja.FunctionCall) goja.Value {
    36			ref, err := m.engineFromDefaults(call)
    37			if err != nil {
    38				panic(vm.NewGoError(err))
    39			}
    40			return vm.ToValue(ref)
    41		}); err != nil {
    42			panic(vm.NewGoError(err))
    43		}
    44		if err := enginesObj.Set("inspectDefaults", func(call goja.FunctionCall) goja.Value {
    45			info, err := m.inspectEngineDefaults(call)
    46			if err != nil {
    47				panic(vm.NewGoError(err))
    48			}
    49			return vm.ToValue(info)
    50		}); err != nil {
    51			panic(vm.NewGoError(err))
    52		}
    53		if err := exports.Set("engines", enginesObj); err != nil {
    54			panic(vm.NewGoError(err))
    55		}
    56	}
    57	
    58	func (m *module) engineFromDefaults(call goja.FunctionCall) (any, error) {
    59		ss, err := m.cloneInferenceSettingsWithOverrides(call)
    60		if err != nil {
    61			return nil, err
    62		}
    63		eng, err := factory.NewEngineFromSettings(ss)
    64		if err != nil {
    65			return nil, err
    66		}
    67		return eng, nil
    68	}
    69	
    70	func (m *module) inspectEngineDefaults(call goja.FunctionCall) (map[string]any, error) {
    71		ss, err := m.cloneInferenceSettingsWithOverrides(call)
    72		if err != nil {
    73			return nil, err
    74		}
    75		return describeInferenceSettings(ss), nil
    76	}
    77	
    78	func (m *module) cloneInferenceSettingsWithOverrides(call goja.FunctionCall) (*aisettings.InferenceSettings, error) {
    79		if m.opts.DefaultInferenceSettings == nil {
    80			return nil, fmt.Errorf("pinocchio default inference settings are not configured")
    81		}
    82		ss := m.opts.DefaultInferenceSettings.Clone()
    83		if ss == nil {
    84			return nil, fmt.Errorf("pinocchio default inference settings are not available")
    85		}
    86		if len(call.Arguments) > 0 && call.Arguments[0] != nil && !goja.IsUndefined(call.Arguments[0]) && !goja.IsNull(call.Arguments[0]) {
    87			opts, ok := call.Arguments[0].Export().(map[string]any)
    88			if !ok {
    89				return nil, fmt.Errorf("pinocchio.engines.fromDefaults expects an options object")
    90			}
    91			applyEngineOverrides(ss, opts)
    92		}
    93		return ss, nil
    94	}
    95	
    96	func applyEngineOverrides(ss *aisettings.InferenceSettings, opts map[string]any) {
    97		if ss == nil || opts == nil {
    98			return
    99		}
   100		model := strings.TrimSpace(asString(opts["model"]))

## OAuth profile source resolver
    20	const runtimeOAuthRedirectURL = "http://127.0.0.1/oauth/callback"
    21	
    22	// ResolvedOAuthProfile binds one resolved profile to the sole direct YAML
    23	// registry file that owns its secret tuple and its exact outbound request.
    24	type ResolvedOAuthProfile struct {
    25		Profile *oauthprofiles.Profile
    26		Store   *oauthprofiles.YAMLStore
    27		Request credentials.Request
    28	}
    29	
    30	// ResolveOAuthProfile resolves the selected profile's OAuth extension. OAuth
    31	// profiles are deliberately supported only from one direct YAML registry file;
    32	// inline, composed, SQLite, and remote-like sources have no safe write target.
    33	func ResolveOAuthProfile(ctx context.Context, resolved *ResolvedCLIEngineSettings) (*ResolvedOAuthProfile, error) {
    34		if resolved == nil || resolved.ResolvedEngineProfile == nil {
    35			return nil, nil
    36		}
    37		if resolved.ProfileRuntime == nil || resolved.ProfileRuntime.Reader() == nil {
    38			return nil, errors.New("OAuth profile resolution requires a profile registry runtime")
    39		}
    40		if resolved.FinalInferenceSettings == nil {
    41			return nil, errors.New("OAuth profile resolution requires final inference settings")
    42		}
    43	
    44		profile, err := resolved.ProfileRuntime.Reader().GetEngineProfile(ctx, resolved.ResolvedEngineProfile.RegistrySlug, resolved.ResolvedEngineProfile.EngineProfileSlug)
    45		if err != nil {
    46			return nil, fmt.Errorf("load selected OAuth profile: %w", err)
    47		}
    48		oauthProfile, err := oauthprofiles.Parse(profile.Extensions)
    49		if err != nil {
    50			return nil, err
    51		}
    52		if oauthProfile == nil {
    53			return nil, nil
    54		}
    55	
    56		request, err := oauthCredentialRequest(resolved.FinalInferenceSettings)
    57		if err != nil {
    58			return nil, err
    59		}
    60		if err := rejectStaticOAuthCredential(resolved.FinalInferenceSettings, request); err != nil {
    61			return nil, err
    62		}
    63		path, err := directYAMLRegistryPath(resolved.ProfileRuntime.ProfileSettings.ProfileRegistries, resolved.ResolvedEngineProfile.RegistrySlug, resolved.ResolvedEngineProfile.EngineProfileSlug)
    64		if err != nil {
    65			return nil, err
    66		}
    67		store, err := oauthprofiles.NewYAMLStore(path, resolved.ResolvedEngineProfile.RegistrySlug, resolved.ResolvedEngineProfile.EngineProfileSlug, request)
    68		if err != nil {
    69			return nil, err
    70		}
    71		return &ResolvedOAuthProfile{Profile: oauthProfile, Store: store, Request: request}, nil
    72	}
    73	
    74	// NewOAuthClient creates a profile-bound reusable protocol client for the
    75	// caller's exact callback URL. Runtime renewal uses a loopback placeholder
    76	// because refresh grants never use the redirect URL; browser login passes the
    77	// bound listener URL instead.
    78	func (r *ResolvedOAuthProfile) NewOAuthClient(redirectURL string) (*geppettoauth.Client, error) {
    79		if r == nil || r.Profile == nil {
    80			return nil, errors.New("resolved OAuth profile is required")
    81		}
    82		config, err := r.Profile.ProtocolConfig(redirectURL)
    83		if err != nil {
    84			return nil, err
    85		}
    86		return geppettoauth.NewClient(config, geppettoauth.WithRefreshTokenPolicy(r.Profile.RefreshTokenPolicy))
    87	}
    88	
    89	// NewBearerTokenSource constructs the Geppetto renewable source for this
    90	// profile. The store persists a rotated tuple before the source caches it.
    91	func (r *ResolvedOAuthProfile) NewBearerTokenSource() (credentials.BearerTokenSource, error) {
    92		client, err := r.NewOAuthClient(runtimeOAuthRedirectURL)
    93		if err != nil {
    94			return nil, err
    95		}
    96		refresher, err := oauthprofiles.NewRefresher(client)
    97		if err != nil {
    98			return nil, err
    99		}
   100		return credentials.NewRenewableBearerTokenSource(r.Store, refresher)
   101	}
   102	
   103	// NewEngineFactoryForResolvedSettings returns a standard factory with a
   104	// renewable bearer source only when the selected profile explicitly opts into
   105	// OAuth. Static-key profiles retain existing behavior.
   106	func NewEngineFactoryForResolvedSettings(ctx context.Context, resolved *ResolvedCLIEngineSettings) (factory.EngineFactory, error) {
   107		oauthProfile, err := ResolveOAuthProfile(ctx, resolved)
   108		if err != nil {
   109			return nil, err
   110		}
   111		if oauthProfile == nil {
   112			return factory.NewStandardEngineFactory(), nil
   113		}
   114		source, err := oauthProfile.NewBearerTokenSource()
   115		if err != nil {
   116			return nil, err
   117		}
   118		return factory.NewStandardEngineFactory(factory.WithBearerTokenSource(source)), nil
   119	}
   120	
   121	func directYAMLRegistryPath(entries []string, registrySlug gepprofiles.RegistrySlug, profileSlug gepprofiles.EngineProfileSlug) (string, error) {
   122		specs, err := gepprofiles.ParseRegistrySourceSpecs(entries)
   123		if err != nil {
   124			return "", err
   125		}

## Atomic YAML store
    35		}
    36		if registry.IsZero() || profile.IsZero() {
    37			return nil, errors.New("OAuth profile registry and profile slugs are required")
    38		}
    39		if _, err := requestKey(expected); err != nil {
    40			return nil, err
    41		}
    42		return &YAMLStore{
    43			path:     filepath.Clean(path),
    44			registry: registry,
    45			profile:  profile,
    46			expected: normalizeRequest(expected),
    47		}, nil
    48	}
    49	
    50	// Load returns the current persisted credential after checking file security,
    51	// the target profile identity, and the request identity. Errors never include
    52	// OAuth token contents.
    53	func (s *YAMLStore) Load(ctx context.Context, request credentials.Request) (credentials.Credential, error) {
    54		if err := s.validateRequest(request); err != nil {
    55			return credentials.Credential{}, err
    56		}
    57		if err := contextErr(ctx); err != nil {
    58			return credentials.Credential{}, err
    59		}
    60		var credential credentials.Credential
    61		err := withRegistryLock(s.path, false, func() error {
    62			profile, err := s.loadProfile()
    63			if err != nil {
    64				return err
    65			}
    66			parsed, err := Parse(profile.Extensions)
    67			if err != nil {
    68				return err
    69			}
    70			if parsed == nil {
    71				return errors.New("OAuth profile extension is missing")
    72			}
    73			credential = parsed.Credential
    74			return nil
    75		})
    76		if err != nil {
    77			return credentials.Credential{}, err
    78		}
    79		return credential, nil
    80	}
    81	
    82	// Save atomically replaces access, refresh, and expiry state as one tuple. A
    83	// refresh token is always required: Geppetto's refresh policy has already
    84	// chosen whether an omitted provider token was preserved or rejected.
    85	func (s *YAMLStore) Save(ctx context.Context, request credentials.Request, credential credentials.Credential) error {
    86		if err := s.validateRequest(request); err != nil {
    87			return err
    88		}
    89		if err := contextErr(ctx); err != nil {
    90			return err
    91		}
    92		if strings.TrimSpace(credential.AccessToken) == "" {
    93			return errors.New("OAuth credential access token is required")
    94		}
    95		if strings.TrimSpace(credential.RefreshToken) == "" {
    96			return errors.New("OAuth credential refresh token is required")
    97		}
    98	
    99		return withRegistryLock(s.path, true, func() error {
   100			registry, profile, err := s.loadRegistryAndProfile()
   101			if err != nil {
   102				return err
   103			}
   104			parsed, err := Parse(profile.Extensions)
   105			if err != nil {
   106				return err
   107			}
   108			if parsed == nil {
   109				return errors.New("OAuth profile extension is missing")
   110			}
   111			setCredential(profile.Extensions, credential)
   112			registry.Profiles[s.profile] = profile
   113			data, err := gepprofiles.EncodeEngineProfileYAMLSingleRegistry(registry)
   114			if err != nil {
   115				return fmt.Errorf("encode OAuth profile registry: %w", err)
   116			}
   117			return atomicWriteOwnerOnly(s.path, data)
   118		})
   119	}
   120	
   121	// Path returns the direct YAML registry path, for diagnostics that must never
   122	// include credential values.
   123	func (s *YAMLStore) Path() string {
   124		if s == nil {
   125			return ""
   126		}
   127		return s.path
   128	}
   129	
   130	func (s *YAMLStore) validateRequest(request credentials.Request) error {
   131		if s == nil {
   132			return errors.New("nil OAuth profile credential store")
   133		}
   134		if _, err := requestKey(request); err != nil {
   135			return err
   136		}
   137		if normalizeRequest(request) != s.expected {
   138			return errors.New("OAuth credential request does not match the selected profile")
   139		}
   140		return nil
   141	}
   142	
   143	func (s *YAMLStore) loadProfile() (*gepprofiles.EngineProfile, error) {
   144		_, profile, err := s.loadRegistryAndProfile()
   145		return profile, err
   146	}
   147	
   148	func (s *YAMLStore) loadRegistryAndProfile() (*gepprofiles.EngineProfileRegistry, *gepprofiles.EngineProfile, error) {
   149		if err := ensureOwnerOnlyRegistry(s.path); err != nil {
   150			return nil, nil, err
   151		}
   152		data, err := os.ReadFile(s.path)
   153		if err != nil {
   154			return nil, nil, fmt.Errorf("read OAuth profile registry: %w", err)
   155		}
   156		registry, err := gepprofiles.DecodeEngineProfileYAMLSingleRegistry(data)
   157		if err != nil {
   158			return nil, nil, fmt.Errorf("decode OAuth profile registry: %w", err)
   159		}
   160		if registry == nil || registry.Slug != s.registry {
   161			return nil, nil, errors.New("OAuth profile registry does not match selected registry")
   162		}
   163		profile := registry.Profiles[s.profile]
   164		if profile == nil {
   165			return nil, nil, errors.New("OAuth profile does not exist in selected registry")
   166		}
   167		return registry, profile, nil
   168	}
   169	
   170	func setCredential(extensions map[string]any, credential credentials.Credential) {
   171		oauth, _ := stringAnyMap(extensions[ExtensionKey])
   172		if oauth == nil {
   173			oauth = map[string]any{}
   174		}
   175		oauth["access_token"] = credential.AccessToken
   176		oauth["refresh_token"] = credential.RefreshToken
   177		if credential.ExpiresAt.IsZero() {
   178			delete(oauth, "expires_at")
   179		} else {
   180			oauth["expires_at"] = credential.ExpiresAt.UTC().Format(time.RFC3339)
   181		}
   182		extensions[ExtensionKey] = oauth
   183	}
   184	
   185	func ensureOwnerOnlyRegistry(path string) error {
   186		info, err := os.Lstat(path)
   187		if err != nil {
   188			return fmt.Errorf("stat OAuth profile registry: %w", err)
   189		}
   190		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
   191			return errors.New("OAuth profile registry must be a regular file")
   192		}
   193		if info.Mode().Perm() != 0o600 {
   194			return errors.New("OAuth profile registry must have mode 0600")
   195		}
   196		dirInfo, err := os.Stat(filepath.Dir(path))
   197		if err != nil {
   198			return fmt.Errorf("stat OAuth profile registry directory: %w", err)
   199		}
   200		if !dirInfo.IsDir() || dirInfo.Mode().Perm()&0o022 != 0 {
   201			return errors.New("OAuth profile registry directory must not be group or world writable")
   202		}
   203		return nil
   204	}
   205	
   206	func atomicWriteOwnerOnly(path string, data []byte) error {
   207		dir := filepath.Dir(path)
   208		tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".oauth-")
   209		if err != nil {
   210			return fmt.Errorf("create OAuth profile temporary file: %w", err)
   211		}
   212		tmpPath := tmp.Name()
   213		completed := false
   214		defer func() {
   215			if !completed {
   216				_ = os.Remove(tmpPath)
   217			}
   218		}()
   219		if err := tmp.Chmod(0o600); err != nil {
   220			_ = tmp.Close()
   221			return fmt.Errorf("set OAuth profile temporary file mode: %w", err)
   222		}
   223		if _, err := tmp.Write(data); err != nil {
   224			_ = tmp.Close()
   225			return fmt.Errorf("write OAuth profile temporary file: %w", err)
