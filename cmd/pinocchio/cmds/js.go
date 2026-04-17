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
	geppettobootstrap "github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
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
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	pjs "github.com/go-go-golems/pinocchio/pkg/js/modules/pinocchio"
	"github.com/spf13/cobra"
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
	inferenceDebugSection, err := geppettobootstrap.NewInferenceDebugSection()
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
	configFiles, err := profilebootstrap.ResolveCLIConfigFilesResolved(parsedCommandSections)
	if err != nil {
		return nil, err
	}

	return []cmd_sources.Middleware{
		cmd_sources.FromCobra(cmd, fields.WithSource("cobra")),
		cmd_sources.FromArgs(args, fields.WithSource("arguments")),
		cmd_sources.FromEnv("PINOCCHIO", fields.WithSource("env")),
		cmd_sources.FromResolvedFiles(
			configFiles.Files,
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
		debugSettings := &geppettobootstrap.InferenceDebugSettings{}
		if err := parsed.DecodeSectionInto(geppettobootstrap.InferenceDebugSectionSlug, debugSettings); err != nil {
			return err
		}
		if debugSettings.PrintInferenceSettings {
			_, err := geppettobootstrap.HandleInferenceDebugOutput(
				w,
				profilebootstrap.BootstrapConfig(),
				parsed,
				*debugSettings,
				runtimeBootstrap.ResolvedEngineSettings,
				geppettobootstrap.InferenceDebugOutputOptions{},
			)
			return err
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

	unifiedConfig, err := profilebootstrap.ResolveUnifiedConfig(parsed)
	if err != nil {
		if resolved.Close != nil {
			resolved.Close()
		}
		return nil, err
	}
	registryChain, err := profilebootstrap.ResolveUnifiedProfileRegistryChain(ctx, unifiedConfig)
	if err != nil {
		if resolved.Close != nil {
			resolved.Close()
		}
		return nil, err
	}

	var profileRegistry gepprofiles.RegistryReader
	var useDefaultProfileResolve bool
	var defaultProfileResolve gepprofiles.ResolveInput
	if registryChain != nil {
		profileRegistry = registryChain.Reader
		useDefaultProfileResolve = registryChain.DefaultProfileResolve.EngineProfileSlug != ""
		defaultProfileResolve = registryChain.DefaultProfileResolve
	}

	return &pinocchioJSRuntimeBootstrap{
		DefaultInferenceSettings: resolved.FinalInferenceSettings,
		ResolvedEngineSettings:   resolved,
		ProfileRegistry:          profileRegistry,
		UseDefaultProfileResolve: useDefaultProfileResolve,
		DefaultProfileResolve:    defaultProfileResolve,
		Close: func() {
			if registryChain != nil && registryChain.Close != nil {
				registryChain.Close()
			}
			if resolved.Close != nil {
				resolved.Close()
			}
		},
	}, nil
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
