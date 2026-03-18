package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	gp "github.com/go-go-golems/geppetto/pkg/js/modules/geppetto"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	gojengine "github.com/go-go-golems/go-go-goja/engine"
	agenttools "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
	cmdhelpers "github.com/go-go-golems/pinocchio/pkg/cmds/helpers"
	pjs "github.com/go-go-golems/pinocchio/pkg/js/modules/pinocchio"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	sqlitetool "github.com/go-go-golems/pinocchio/pkg/middlewares/sqlitetool"
	"github.com/spf13/cobra"
)

func NewJSCommand() *cobra.Command {
	var (
		scriptPath  string
		profile     string
		printResult bool
		listGoTools bool
	)

	cmd := &cobra.Command{
		Use:   "js [script.js]",
		Short: "Run JavaScript against Pinocchio's Geppetto runtime",
		Args: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(scriptPath) != "" && len(args) > 0 {
				return fmt.Errorf("provide either --script or a positional script path, not both")
			}
			if len(args) > 1 {
				return fmt.Errorf("expected at most one positional script path")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(scriptPath) == "" && len(args) == 1 {
				scriptPath = args[0]
			}
			return runJSCommand(cmd.Context(), cmd, jsCommandSettings{
				ScriptPath:  scriptPath,
				PrintResult: printResult,
				ListGoTools: listGoTools,
			})
		},
	}

	cmd.Flags().StringVar(&scriptPath, "script", "", "Path to JavaScript file to execute")
	cmd.Flags().String("config-file", "", "Path to Pinocchio config file")
	cmd.Flags().StringVar(&profile, "profile", "", "Load this profile from the configured profile registries")
	cmd.Flags().BoolVar(&printResult, "print-result", false, "Print top-level JS return value as JSON")
	cmd.Flags().BoolVar(&listGoTools, "list-go-tools", false, "List built-in Go tools exposed to JS and exit")
	return cmd
}

type jsCommandSettings struct {
	ScriptPath  string
	PrintResult bool
	ListGoTools bool
}

func runJSCommand(ctx context.Context, cmd *cobra.Command, settings jsCommandSettings) error {
	goRegistry, err := buildPinocchioJSToolRegistry()
	if err != nil {
		return err
	}
	if settings.ListGoTools {
		for _, td := range goRegistry.ListTools() {
			fmt.Fprintln(cmd.OutOrStdout(), td.Name)
		}
		return nil
	}

	scriptPath := strings.TrimSpace(settings.ScriptPath)
	if scriptPath == "" {
		return fmt.Errorf("--script is required")
	}
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		return err
	}

	parsed, err := buildJSParsedValues(cmd)
	if err != nil {
		return err
	}
	baseStepSettings, _, err := cmdhelpers.ResolveBaseStepSettings(parsed)
	if err != nil {
		return err
	}
	profileRegistry, defaultProfileResolve, closer, err := loadPinocchioProfileRegistryStack(parsed)
	if err != nil {
		return err
	}
	if closer != nil {
		defer func() {
			_ = closer.Close()
		}()
	}
	middlewareDefs, buildDeps, err := buildPinocchioJSMiddlewareRegistry()
	if err != nil {
		return err
	}
	middlewareFactories := buildPinocchioJSMiddlewareFactories(buildDeps)

	scriptDir := filepath.Dir(scriptPath)
	rt, err := newPinocchioJSRuntime(ctx, pinocchioJSRuntimeOptions{
		ScriptDir:                scriptDir,
		BaseStepSettings:         baseStepSettings,
		GoToolRegistry:           goRegistry,
		ProfileRegistry:          profileRegistry,
		UseDefaultProfileResolve: profileRegistry != nil,
		DefaultProfileResolve:    defaultProfileResolve,
		GoMiddlewareFactories:    middlewareFactories,
		MiddlewareDefinitions:    middlewareDefs,
	})
	if err != nil {
		return err
	}
	defer func() {
		_ = rt.Close(context.Background())
	}()

	result, err := rt.VM.RunScript(filepath.Base(scriptPath), string(scriptBytes))
	if err != nil {
		return err
	}
	if settings.PrintResult && result != nil && !goja.IsUndefined(result) && !goja.IsNull(result) {
		b, err := json.MarshalIndent(result.Export(), "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
	}
	return nil
}

func buildJSParsedValues(cmd *cobra.Command) (*values.Values, error) {
	ret := values.New()
	commandSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}
	profileSection, err := cmdhelpers.NewProfileSettingsSection()
	if err != nil {
		return nil, err
	}

	commandValues, err := values.NewSectionValues(commandSection)
	if err != nil {
		return nil, err
	}
	configFile := strings.TrimSpace(inheritedStringFlag(cmd, "config-file"))
	if configFile == "" {
		configFile = strings.TrimSpace(localStringFlag(cmd, "config-file"))
	}
	if configFile != "" {
		if err := values.WithFieldValue("config-file", configFile)(commandValues); err != nil {
			return nil, err
		}
	}
	ret.Set(cli.CommandSettingsSlug, commandValues)

	profileValues, err := values.NewSectionValues(profileSection)
	if err != nil {
		return nil, err
	}
	if raw := strings.TrimSpace(inheritedStringFlag(cmd, "profile-registries")); raw != "" {
		if err := values.WithFieldValue("profile-registries", raw, fields.WithSource("cli"))(profileValues); err != nil {
			return nil, err
		}
	}
	if raw := strings.TrimSpace(localStringFlag(cmd, "profile")); raw != "" {
		if err := values.WithFieldValue("profile", raw, fields.WithSource("cli"))(profileValues); err != nil {
			return nil, err
		}
	}
	ret.Set(cmdhelpers.ProfileSettingsSectionSlug, profileValues)
	return ret, nil
}

func inheritedStringFlag(cmd *cobra.Command, name string) string {
	if cmd == nil {
		return ""
	}
	if f := cmd.Flags().Lookup(name); f != nil {
		return strings.TrimSpace(f.Value.String())
	}
	if f := cmd.InheritedFlags().Lookup(name); f != nil {
		return strings.TrimSpace(f.Value.String())
	}
	return ""
}

func localStringFlag(cmd *cobra.Command, name string) string {
	if cmd == nil {
		return ""
	}
	if f := cmd.Flags().Lookup(name); f != nil {
		return strings.TrimSpace(f.Value.String())
	}
	return ""
}

func loadPinocchioProfileRegistryStack(parsed *values.Values) (gepprofiles.RegistryReader, gepprofiles.ResolveInput, io.Closer, error) {
	profileSettings, _, err := cmdhelpers.ResolveEffectiveProfileSettings(parsed)
	if err != nil {
		return nil, gepprofiles.ResolveInput{}, nil, err
	}
	if profileSettings.ProfileRegistries == "" {
		return nil, gepprofiles.ResolveInput{}, nil, nil
	}
	entries, err := gepprofiles.ParseProfileRegistrySourceEntries(profileSettings.ProfileRegistries)
	if err != nil {
		return nil, gepprofiles.ResolveInput{}, nil, err
	}
	specs, err := gepprofiles.ParseRegistrySourceSpecs(entries)
	if err != nil {
		return nil, gepprofiles.ResolveInput{}, nil, err
	}
	chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(context.Background(), specs)
	if err != nil {
		return nil, gepprofiles.ResolveInput{}, nil, err
	}
	defaultResolve := gepprofiles.ResolveInput{}
	var reader gepprofiles.RegistryReader = chain
	if profileSettings.Profile != "" {
		profileSlug, err := gepprofiles.ParseProfileSlug(profileSettings.Profile)
		if err != nil {
			_ = chain.Close()
			return nil, gepprofiles.ResolveInput{}, nil, err
		}
		defaultResolve.ProfileSlug = profileSlug
		reader = selectedProfileRegistryReader{
			base:            chain,
			selectedProfile: profileSlug,
		}
	}
	return reader, defaultResolve, chain, nil
}

type selectedProfileRegistryReader struct {
	base            gepprofiles.RegistryReader
	selectedProfile gepprofiles.ProfileSlug
}

func (r selectedProfileRegistryReader) ListRegistries(ctx context.Context) ([]gepprofiles.RegistrySummary, error) {
	return r.base.ListRegistries(ctx)
}

func (r selectedProfileRegistryReader) GetRegistry(ctx context.Context, registrySlug gepprofiles.RegistrySlug) (*gepprofiles.ProfileRegistry, error) {
	return r.base.GetRegistry(ctx, registrySlug)
}

func (r selectedProfileRegistryReader) ListProfiles(ctx context.Context, registrySlug gepprofiles.RegistrySlug) ([]*gepprofiles.Profile, error) {
	return r.base.ListProfiles(ctx, registrySlug)
}

func (r selectedProfileRegistryReader) GetProfile(ctx context.Context, registrySlug gepprofiles.RegistrySlug, profileSlug gepprofiles.ProfileSlug) (*gepprofiles.Profile, error) {
	return r.base.GetProfile(ctx, registrySlug, profileSlug)
}

func (r selectedProfileRegistryReader) ResolveEffectiveProfile(ctx context.Context, in gepprofiles.ResolveInput) (*gepprofiles.ResolvedProfile, error) {
	if in.ProfileSlug == "" && r.selectedProfile != "" {
		in.ProfileSlug = r.selectedProfile
	}
	return r.base.ResolveEffectiveProfile(ctx, in)
}

type pinocchioJSRuntimeOptions struct {
	ScriptDir                string
	BaseStepSettings         *aisettings.StepSettings
	GoToolRegistry           geptools.ToolRegistry
	ProfileRegistry          gepprofiles.RegistryReader
	UseDefaultProfileResolve bool
	DefaultProfileResolve    gepprofiles.ResolveInput
	GoMiddlewareFactories    map[string]gp.MiddlewareFactory
	MiddlewareDefinitions    middlewarecfg.DefinitionRegistry
}

func newPinocchioJSRuntime(ctx context.Context, opts pinocchioJSRuntimeOptions) (*gojengine.Runtime, error) {
	requireOpts := []require.Option{
		require.WithGlobalFolders(
			opts.ScriptDir,
			filepath.Join(opts.ScriptDir, "node_modules"),
		),
	}
	builder := gojengine.NewBuilder(gojengine.WithRequireOptions(requireOpts...))
	factory, err := builder.Build()
	if err != nil {
		return nil, err
	}
	rt, err := factory.NewRuntime(ctx)
	if err != nil {
		return nil, err
	}

	reg := require.NewRegistry(requireOpts...)
	gp.Register(reg, gp.Options{
		Runner:                   rt.Owner,
		GoToolRegistry:           opts.GoToolRegistry,
		GoMiddlewareFactories:    opts.GoMiddlewareFactories,
		ProfileRegistry:          opts.ProfileRegistry,
		UseDefaultProfileResolve: opts.UseDefaultProfileResolve,
		DefaultProfileResolve:    opts.DefaultProfileResolve,
		MiddlewareSchemas:        opts.MiddlewareDefinitions,
	})
	pjs.Register(reg, pjs.Options{
		BaseStepSettings: opts.BaseStepSettings,
	})
	req := reg.Enable(rt.VM)
	rt.Require = req

	runtimeCtx := &gojengine.RuntimeContext{
		VM:      rt.VM,
		Require: req,
		Loop:    rt.Loop,
		Owner:   rt.Owner,
	}
	if err := (runtimeInitializerFunc{
		id: "pinocchio-js-helpers",
		fn: func(_ *gojengine.RuntimeContext) error {
			installConsole(rt.VM)
			installHelpers(rt.VM)
			return nil
		},
	}).InitRuntime(runtimeCtx); err != nil {
		_ = rt.Close(context.Background())
		return nil, err
	}
	return rt, nil
}

type runtimeInitializerFunc struct {
	id string
	fn func(ctx *gojengine.RuntimeContext) error
}

func (f runtimeInitializerFunc) ID() string { return f.id }

func (f runtimeInitializerFunc) InitRuntime(ctx *gojengine.RuntimeContext) error {
	if f.fn == nil {
		return nil
	}
	return f.fn(ctx)
}

func installHelpers(vm *goja.Runtime) {
	_ = vm.Set("ENV", mapEnv())
	_ = vm.Set("sleep", func(ms int64) {
		if ms <= 0 {
			return
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
	})
	_, err := vm.RunString(`
globalThis.assert = function assert(cond, msg) {
  if (!cond) {
    throw new Error(msg || "assertion failed");
  }
};
`)
	if err != nil {
		panic(err)
	}
}

func installConsole(vm *goja.Runtime) {
	console := vm.NewObject()
	_ = console.Set("log", func(call goja.FunctionCall) goja.Value {
		fmt.Fprintln(os.Stdout, joinArgs(call.Arguments))
		return goja.Undefined()
	})
	_ = console.Set("error", func(call goja.FunctionCall) goja.Value {
		fmt.Fprintln(os.Stderr, joinArgs(call.Arguments))
		return goja.Undefined()
	})
	_ = vm.Set("console", console)
}

func mapEnv() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		out[parts[0]] = parts[1]
	}
	return out
}

func joinArgs(args []goja.Value) string {
	if len(args) == 0 {
		return ""
	}
	out := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case arg == nil || goja.IsUndefined(arg):
			out = append(out, "undefined")
		case goja.IsNull(arg):
			out = append(out, "null")
		default:
			exp := arg.Export()
			if b, err := json.Marshal(exp); err == nil && json.Valid(b) {
				out = append(out, string(b))
			} else {
				out = append(out, fmt.Sprint(exp))
			}
		}
	}
	return strings.Join(out, " ")
}

func buildPinocchioJSToolRegistry() (*geptools.InMemoryToolRegistry, error) {
	reg := geptools.NewInMemoryToolRegistry()
	if err := agenttools.RegisterCalculatorTool(reg); err != nil {
		return nil, err
	}
	return reg, nil
}

func buildPinocchioJSMiddlewareRegistry() (*middlewarecfg.InMemoryDefinitionRegistry, middlewarecfg.BuildDeps, error) {
	registry := middlewarecfg.NewInMemoryDefinitionRegistry()
	defs := []middlewarecfg.Definition{
		jsAgentModeMiddlewareDefinition(),
		jsSQLiteMiddlewareDefinition(),
	}
	for _, def := range defs {
		if err := registry.RegisterDefinition(def); err != nil {
			return nil, middlewarecfg.BuildDeps{}, err
		}
	}
	return registry, middlewarecfg.BuildDeps{
		Values: map[string]any{
			"agentmode.service": agentmode.NewStaticService(nil),
		},
	}, nil
}

func buildPinocchioJSMiddlewareFactories(deps middlewarecfg.BuildDeps) map[string]gp.MiddlewareFactory {
	return map[string]gp.MiddlewareFactory{
		"agentmode": func(options map[string]any) (middleware.Middleware, error) {
			return jsAgentModeMiddlewareDefinition().Build(context.Background(), deps.Clone(), options)
		},
		"sqlite": func(options map[string]any) (middleware.Middleware, error) {
			return jsSQLiteMiddlewareDefinition().Build(context.Background(), deps.Clone(), options)
		},
	}
}

type jsMiddlewareDefinition struct {
	name        string
	schema      map[string]any
	build       func(context.Context, middlewarecfg.BuildDeps, any) (middleware.Middleware, error)
	description string
}

func (d jsMiddlewareDefinition) Name() string { return d.name }
func (d jsMiddlewareDefinition) ConfigJSONSchema() map[string]any {
	return cloneStringAnyMap(d.schema)
}
func (d jsMiddlewareDefinition) Build(ctx context.Context, deps middlewarecfg.BuildDeps, cfg any) (middleware.Middleware, error) {
	return d.build(ctx, deps, cfg)
}

func jsAgentModeMiddlewareDefinition() middlewarecfg.Definition {
	type configInput struct {
		DefaultMode string `json:"default_mode,omitempty"`
	}
	return jsMiddlewareDefinition{
		name:        "agentmode",
		description: "Parses and applies agent-mode switches from model output.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"default_mode": map[string]any{"type": "string", "default": agentmode.DefaultConfig().DefaultMode},
			},
			"additionalProperties": false,
		},
		build: func(_ context.Context, deps middlewarecfg.BuildDeps, cfg any) (middleware.Middleware, error) {
			svcRaw, ok := deps.Get("agentmode.service")
			if !ok || svcRaw == nil {
				return nil, fmt.Errorf("missing dependency %q", "agentmode.service")
			}
			svc, ok := svcRaw.(agentmode.Service)
			if !ok {
				return nil, fmt.Errorf("dependency %q has unexpected type %T", "agentmode.service", svcRaw)
			}
			input := configInput{DefaultMode: agentmode.DefaultConfig().DefaultMode}
			if err := decodeResolvedMiddlewareConfig(cfg, &input); err != nil {
				return nil, err
			}
			config := agentmode.DefaultConfig()
			if s := strings.TrimSpace(input.DefaultMode); s != "" {
				config.DefaultMode = s
			}
			return agentmode.NewMiddleware(svc, config), nil
		},
	}
}

func jsSQLiteMiddlewareDefinition() middlewarecfg.Definition {
	type configInput struct {
		DSN                *string `json:"dsn,omitempty"`
		MaxRows            *int    `json:"max_rows,omitempty"`
		ExecutionTimeoutMs *int64  `json:"execution_timeout_ms,omitempty"`
		MaxOutputLines     *int    `json:"max_output_lines,omitempty"`
		MaxOutputBytes     *int    `json:"max_output_bytes,omitempty"`
	}
	return jsMiddlewareDefinition{
		name:        "sqlite",
		description: "Executes SQL tool calls against configured SQLite connection settings.",
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dsn":                  map[string]any{"type": "string"},
				"max_rows":             map[string]any{"type": "integer", "minimum": 1},
				"execution_timeout_ms": map[string]any{"type": "integer", "minimum": 1},
				"max_output_lines":     map[string]any{"type": "integer", "minimum": 1},
				"max_output_bytes":     map[string]any{"type": "integer", "minimum": 1},
			},
			"additionalProperties": false,
		},
		build: func(_ context.Context, _ middlewarecfg.BuildDeps, cfg any) (middleware.Middleware, error) {
			config := sqlitetool.DefaultConfig()
			var input configInput
			if err := decodeResolvedMiddlewareConfig(cfg, &input); err != nil {
				return nil, err
			}
			if input.DSN != nil {
				config.DSN = strings.TrimSpace(*input.DSN)
			}
			if input.MaxRows != nil {
				config.MaxRows = *input.MaxRows
			}
			if input.ExecutionTimeoutMs != nil {
				config.ExecutionTimeout = time.Duration(*input.ExecutionTimeoutMs) * time.Millisecond
			}
			if input.MaxOutputLines != nil {
				config.MaxOutputLines = *input.MaxOutputLines
			}
			if input.MaxOutputBytes != nil {
				config.MaxOutputBytes = *input.MaxOutputBytes
			}
			return sqlitetool.NewMiddleware(config), nil
		},
	}
}

func decodeResolvedMiddlewareConfig(cfg any, out any) error {
	if cfg == nil || out == nil {
		return nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serialize resolved middleware config: %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode resolved middleware config: %w", err)
	}
	return nil
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if nested, ok := value.(map[string]any); ok {
			out[key] = cloneStringAnyMap(nested)
			continue
		}
		out[key] = value
	}
	return out
}
