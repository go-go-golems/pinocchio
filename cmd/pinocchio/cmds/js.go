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
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	gp "github.com/go-go-golems/geppetto/pkg/js/modules/geppetto"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	cmd_sources "github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	gojengine "github.com/go-go-golems/go-go-goja/engine"
	agenttools "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	pjs "github.com/go-go-golems/pinocchio/pkg/js/modules/pinocchio"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewJSCommand() *cobra.Command {
	command, err := newJSCommand()
	if err != nil {
		panic(err)
	}
	cobraCommand, err := cli.BuildCobraCommand(command, cli.WithParserConfig(cli.CobraParserConfig{
		MiddlewaresFunc: jsCobraMiddlewares,
	}))
	if err != nil {
		panic(err)
	}
	return cobraCommand
}

type JSSettings struct {
	ScriptPath  string `glazed:"script"`
	ScriptArg   string `glazed:"script_path"`
	PrintResult bool   `glazed:"print-result"`
	ListGoTools bool   `glazed:"list-go-tools"`
}

type JSCommand struct {
	*cmds.CommandDescription
}

var _ cmds.WriterCommand = &JSCommand{}

func newJSCommand() (*JSCommand, error) {
	baseSections, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		return nil, err
	}
	inferenceDebugSection, err := cmdlayers.NewInferenceDebugParameterLayer()
	if err != nil {
		return nil, err
	}
	commandOptions := []cmds.CommandDescriptionOption{
		cmds.WithShort("Run JavaScript against Pinocchio's Geppetto runtime"),
		cmds.WithLong("Run JavaScript against Pinocchio's Geppetto runtime"),
		cmds.WithFlags(
			fields.New(
				"script",
				fields.TypeString,
				fields.WithHelp("Path to JavaScript file to execute"),
			),
			fields.New(
				"print-result",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Print top-level JS return value as JSON"),
			),
			fields.New(
				"list-go-tools",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("List built-in Go tools exposed to JS and exit"),
			),
		),
		cmds.WithArguments(
			fields.New(
				"script_path",
				fields.TypeString,
				fields.WithHelp("Path to JavaScript file to execute"),
			),
		),
		cmds.WithSections(inferenceDebugSection),
	}
	commandOptions = append(commandOptions, cmds.WithSections(baseSections...))

	return &JSCommand{
		CommandDescription: cmds.NewCommandDescription("js", commandOptions...),
	}, nil
}

func jsCobraMiddlewares(parsedCommandSections *values.Values, cmd *cobra.Command, args []string) ([]cmd_sources.Middleware, error) {
	configFiles, err := profilebootstrap.ResolveCLIConfigFiles(parsedCommandSections)
	if err != nil {
		return nil, err
	}

	return []cmd_sources.Middleware{
		cmd_sources.FromCobra(cmd, fields.WithSource("cobra")),
		cmd_sources.FromArgs(args, fields.WithSource("arguments")),
		cmd_sources.FromEnv("PINOCCHIO", fields.WithSource("env")),
		cmd_sources.FromFiles(
			configFiles,
			cmd_sources.WithConfigFileMapper(profilebootstrap.MapPinocchioConfigFile),
			cmd_sources.WithParseOptions(fields.WithSource("config")),
		),
		cmd_sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	}, nil
}

func (c *JSCommand) RunIntoWriter(ctx context.Context, parsed *values.Values, w io.Writer) error {
	settings := &JSSettings{}
	if err := parsed.DecodeSectionInto(values.DefaultSlug, settings); err != nil {
		return err
	}
	scriptPath, err := resolveJSScriptPath(settings)
	if err != nil {
		return err
	}

	goRegistry, err := buildPinocchioJSToolRegistry()
	if err != nil {
		return err
	}
	if settings.ListGoTools {
		for _, td := range goRegistry.ListTools() {
			fmt.Fprintln(w, td.Name)
		}
		return nil
	}
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		return err
	}
	runtimeBootstrap, err := resolvePinocchioJSRuntimeBootstrap(ctx, parsed)
	if err != nil {
		return err
	}
	if runtimeBootstrap.Close != nil {
		defer runtimeBootstrap.Close()
	}
	if runtimeBootstrap.ResolvedEngineSettings != nil {
		debugSettings := &cmdlayers.HelpersSettings{}
		if err := parsed.DecodeSectionInto(cmdlayers.GeppettoHelpersSlug, debugSettings); err != nil {
			return err
		}
		if debugSettings.PrintInferenceSources {
			trace, err := profilebootstrap.BuildInferenceSettingsSourceTrace(nil, parsed, runtimeBootstrap.ResolvedEngineSettings)
			if err != nil {
				return err
			}
			encoder := yaml.NewEncoder(w)
			defer func() {
				_ = encoder.Close()
			}()
			return encoder.Encode(trace)
		}
		if debugSettings.PrintInferenceSettings {
			encoder := yaml.NewEncoder(w)
			defer func() {
				_ = encoder.Close()
			}()
			return encoder.Encode(runtimeBootstrap.ResolvedEngineSettings.FinalInferenceSettings)
		}
	}
	middlewareDefs, buildDeps, err := buildPinocchioJSMiddlewareRegistry()
	if err != nil {
		return err
	}
	middlewareFactories := buildPinocchioJSMiddlewareFactories(buildDeps)

	scriptDir := filepath.Dir(scriptPath)
	rt, err := newPinocchioJSRuntime(ctx, pinocchioJSRuntimeOptions{
		ScriptDir:                scriptDir,
		DefaultInferenceSettings: runtimeBootstrap.DefaultInferenceSettings,
		GoToolRegistry:           goRegistry,
		ProfileRegistry:          runtimeBootstrap.ProfileRegistry,
		UseDefaultProfileResolve: runtimeBootstrap.UseDefaultProfileResolve,
		DefaultProfileResolve:    runtimeBootstrap.DefaultProfileResolve,
		GoMiddlewareFactories:    middlewareFactories,
		MiddlewareDefinitions:    middlewareDefs,
		Stdout:                   w,
		Stderr:                   os.Stderr,
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
		fmt.Fprintln(w, string(b))
	}
	return nil
}

func resolveJSScriptPath(settings *JSSettings) (string, error) {
	flagPath := strings.TrimSpace(settings.ScriptPath)
	argPath := strings.TrimSpace(settings.ScriptArg)
	if flagPath != "" && argPath != "" {
		return "", fmt.Errorf("provide either --script or a positional script path, not both")
	}
	if flagPath != "" {
		return flagPath, nil
	}
	if argPath != "" {
		return argPath, nil
	}
	return "", fmt.Errorf("--script is required")
}

func loadPinocchioProfileRegistryStack(parsed *values.Values) (gepprofiles.RegistryReader, gepprofiles.ResolveInput, io.Closer, error) {
	profileSettings, _, err := profilebootstrap.ResolveEngineProfileSettings(parsed)
	if err != nil {
		return nil, gepprofiles.ResolveInput{}, nil, err
	}
	return loadPinocchioProfileRegistryStackFromSettings(profileSettings)
}

func loadPinocchioProfileRegistryStackFromSettings(profileSettings profilebootstrap.ProfileSettings) (gepprofiles.RegistryReader, gepprofiles.ResolveInput, io.Closer, error) {
	if len(profileSettings.ProfileRegistries) == 0 {
		if profileSettings.Profile != "" {
			return nil, gepprofiles.ResolveInput{}, nil, &gepprofiles.ValidationError{
				Field:  "profile-settings.profile-registries",
				Reason: "must be configured when profile-settings.profile is set",
			}
		}
		return nil, gepprofiles.ResolveInput{}, nil, nil
	}
	specs, err := gepprofiles.ParseRegistrySourceSpecs(profileSettings.ProfileRegistries)
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
		profileSlug, err := gepprofiles.ParseEngineProfileSlug(profileSettings.Profile)
		if err != nil {
			_ = chain.Close()
			return nil, gepprofiles.ResolveInput{}, nil, err
		}
		defaultResolve.EngineProfileSlug = profileSlug
		reader = selectedProfileRegistryReader{
			base:            chain,
			selectedProfile: profileSlug,
		}
	}
	return reader, defaultResolve, chain, nil
}

type pinocchioJSRuntimeBootstrap struct {
	DefaultInferenceSettings *aisettings.InferenceSettings
	ResolvedEngineSettings   *profilebootstrap.ResolvedCLIEngineSettings
	ProfileRegistry          gepprofiles.RegistryReader
	UseDefaultProfileResolve bool
	DefaultProfileResolve    gepprofiles.ResolveInput
	Close                    func()
}

func resolvePinocchioJSRuntimeBootstrap(ctx context.Context, parsed *values.Values) (*pinocchioJSRuntimeBootstrap, error) {
	resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsed)
	if err != nil {
		return nil, err
	}

	profileRegistry, defaultResolve, registryCloser, err := loadPinocchioProfileRegistryStackFromSettings(profilebootstrap.ProfileSettings{
		Profile:           resolved.ProfileSelection.Profile,
		ProfileRegistries: append([]string(nil), resolved.ProfileSelection.ProfileRegistries...),
	})
	if err != nil {
		if resolved.Close != nil {
			resolved.Close()
		}
		return nil, err
	}

	return &pinocchioJSRuntimeBootstrap{
		DefaultInferenceSettings: resolved.FinalInferenceSettings,
		ResolvedEngineSettings:   resolved,
		ProfileRegistry:          profileRegistry,
		UseDefaultProfileResolve: profileRegistry != nil,
		DefaultProfileResolve:    defaultResolve,
		Close: func() {
			if registryCloser != nil {
				_ = registryCloser.Close()
			}
			if resolved.Close != nil {
				resolved.Close()
			}
		},
	}, nil
}

type selectedProfileRegistryReader struct {
	base            gepprofiles.RegistryReader
	selectedProfile gepprofiles.EngineProfileSlug
}

func (r selectedProfileRegistryReader) ListRegistries(ctx context.Context) ([]gepprofiles.RegistrySummary, error) {
	return r.base.ListRegistries(ctx)
}

func (r selectedProfileRegistryReader) GetRegistry(ctx context.Context, registrySlug gepprofiles.RegistrySlug) (*gepprofiles.EngineProfileRegistry, error) {
	return r.base.GetRegistry(ctx, registrySlug)
}

func (r selectedProfileRegistryReader) ListEngineProfiles(ctx context.Context, registrySlug gepprofiles.RegistrySlug) ([]*gepprofiles.EngineProfile, error) {
	return r.base.ListEngineProfiles(ctx, registrySlug)
}

func (r selectedProfileRegistryReader) GetEngineProfile(ctx context.Context, registrySlug gepprofiles.RegistrySlug, profileSlug gepprofiles.EngineProfileSlug) (*gepprofiles.EngineProfile, error) {
	return r.base.GetEngineProfile(ctx, registrySlug, profileSlug)
}

func (r selectedProfileRegistryReader) ResolveEngineProfile(ctx context.Context, in gepprofiles.ResolveInput) (*gepprofiles.ResolvedEngineProfile, error) {
	if in.EngineProfileSlug == "" && r.selectedProfile != "" {
		in.EngineProfileSlug = r.selectedProfile
	}
	return r.base.ResolveEngineProfile(ctx, in)
}

type pinocchioJSRuntimeOptions struct {
	ScriptDir                string
	DefaultInferenceSettings *aisettings.InferenceSettings
	GoToolRegistry           geptools.ToolRegistry
	ProfileRegistry          gepprofiles.RegistryReader
	UseDefaultProfileResolve bool
	DefaultProfileResolve    gepprofiles.ResolveInput
	GoMiddlewareFactories    map[string]gp.MiddlewareFactory
	MiddlewareDefinitions    middlewarecfg.DefinitionRegistry
	Stdout                   io.Writer
	Stderr                   io.Writer
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
		EngineProfileRegistry:    opts.ProfileRegistry,
		DefaultInferenceSettings: opts.DefaultInferenceSettings,
		UseDefaultProfileResolve: opts.UseDefaultProfileResolve,
		DefaultProfileResolve:    opts.DefaultProfileResolve,
		MiddlewareSchemas:        opts.MiddlewareDefinitions,
	})
	pjs.Register(reg, pjs.Options{
		DefaultInferenceSettings: opts.DefaultInferenceSettings,
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
			installConsole(rt.VM, opts.Stdout, opts.Stderr)
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

func installConsole(vm *goja.Runtime, stdout io.Writer, stderr io.Writer) {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	console := vm.NewObject()
	_ = console.Set("log", func(call goja.FunctionCall) goja.Value {
		fmt.Fprintln(stdout, joinArgs(call.Arguments))
		return goja.Undefined()
	})
	_ = console.Set("error", func(call goja.FunctionCall) goja.Value {
		fmt.Fprintln(stderr, joinArgs(call.Arguments))
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
	defs := []middlewarecfg.Definition{}
	for _, def := range defs {
		if err := registry.RegisterDefinition(def); err != nil {
			return nil, middlewarecfg.BuildDeps{}, err
		}
	}
	return registry, middlewarecfg.BuildDeps{
		Values: map[string]any{},
	}, nil
}

func buildPinocchioJSMiddlewareFactories(deps middlewarecfg.BuildDeps) map[string]gp.MiddlewareFactory {
	return map[string]gp.MiddlewareFactory{}
}
