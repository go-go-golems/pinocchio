---
Title: Pinocchio CLI Verb Migration Guide (Glazed + Profiles + Config)
Slug: pinocchio-cli-profile-bootstrap-migration
Short: Step-by-step guide to build a Pinocchio verb with Glazed flags, profile selection, config loading, and resolved inference settings like pinocchio js.
Topics:
- pinocchio
- cli
- glazed
- profiles
- config
- migration
- tutorial
Commands:
- pinocchio
- js
Flags:
- config-file
- profile
- profile-registries
- print-parsed-fields
- print-inference-settings
- print-inference-settings-sources
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

This tutorial shows how to migrate a Pinocchio CLI verb from raw Cobra flags to the standard Glazed + profile bootstrap path. The concrete reference is the current `pinocchio js` command in `pinocchio/cmd/pinocchio/cmds/js.go`.

The target is not just “support `--profile`.” The target is a verb that behaves like the rest of modern Pinocchio:

- it parses flags and positional arguments through Glazed,
- it loads config through the Pinocchio config-file mapper,
- it supports `--profile` and `--profile-registries`,
- it resolves the final merged `InferenceSettings`,
- and it can print parsed fields and inference-setting provenance for debugging.

## Conceptual Companion

If you are new to the settings lifecycle, read `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md` alongside this tutorial.

That companion topic explains:

- hidden base settings
- stripped parsed-values base settings
- `BaseInferenceSettings` vs `FinalInferenceSettings`
- runtime profile switching without losing the baseline

## What You Are Building

This section explains the end state before you start editing code.

A migrated verb has four layers:

1. A **Glazed command description** that defines the verb’s own flags and arguments.
2. The shared **Geppetto sections** that expose inference flags and profile settings.
3. A **Pinocchio-aware middleware chain** that merges Cobra, arguments, environment, config files, and defaults.
4. A final call to `profilebootstrap.ResolveCLIEngineSettings(...)` so engine creation happens from the fully resolved settings.

The most useful reference files are:

- `pinocchio/cmd/pinocchio/cmds/js.go`
- `pinocchio/pkg/cmds/profilebootstrap`
- `pinocchio/pkg/cmds/cmdlayers/helpers.go`
- `geppetto/pkg/sections`

## Why This Migration Matters

This section explains why the old raw-Cobra style becomes expensive over time.

When each verb parses flags by hand, three bugs show up repeatedly:

- config loading differs from one verb to another,
- profile selection and profile registry handling drift apart,
- and debugging becomes hard because `--print-parsed-fields` and provenance output do not exist.

The shared bootstrap path fixes that. It gives every verb the same answers to the same questions:

- Which config file was loaded?
- Which profile registries were used?
- Which profile actually resolved?
- What are the final `InferenceSettings`?
- Why does a specific field have its current value?

## Step 1: Replace Local Cobra Flags with a Glazed Command Description

This section covers the first structural change: stop declaring the normal flags with `cmd.Flags().StringVar(...)`.

For `pinocchio js`, the local command flags now live in a `CommandDescription`:

```go
type JSCommand struct {
	*cmds.CommandDescription
}

func newJSCommand() (*JSCommand, error) {
	baseSections, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		return nil, err
	}
	inferenceDebugSection, err := cmdlayers.NewInferenceDebugParameterLayer()
	if err != nil {
		return nil, err
	}

	options := []cmds.CommandDescriptionOption{
		cmds.WithShort("Run JavaScript against Pinocchio's Geppetto runtime"),
		cmds.WithFlags(
			fields.New("script", fields.TypeString, fields.WithHelp("Path to JavaScript file to execute")),
			fields.New("print-result", fields.TypeBool, fields.WithDefault(false)),
			fields.New("list-go-tools", fields.TypeBool, fields.WithDefault(false)),
		),
		cmds.WithArguments(
			fields.New("script_path", fields.TypeString, fields.WithHelp("Path to JavaScript file to execute")),
		),
		cmds.WithSections(inferenceDebugSection),
	}
	options = append(options, cmds.WithSections(baseSections...))

	return &JSCommand{
		CommandDescription: cmds.NewCommandDescription("js", options...),
	}, nil
}
```

That does two things:

- the verb-specific flags are now declarative and Glazed-owned,
- and the command becomes compatible with `cli.BuildCobraCommand(...)`.

## Step 2: Reuse the Shared Geppetto Sections

This section explains which section helpers to mount instead of inventing your own profile/config flags.

For a verb that needs engine/profile resolution, start with:

- `geppetto/pkg/sections.CreateGeppettoSections()`

That gives you:

- `ai-chat`
- `ai-client`
- provider sections like `openai-chat`
- `ai-inference`
- `profile-settings`

If you also want the Pinocchio inference debug flags, add:

- `pinocchio/pkg/cmds/cmdlayers.NewInferenceDebugParameterLayer()`

That gives you:

- `--print-inference-settings`
- `--print-inference-settings-sources`

Because `CreateGeppettoSections()` already includes `profile-settings`, you do not need to define your own `--profile` or `--profile-registries` flags.

It also means cross-profile client settings such as `ai-client.*` are part of the same shared baseline surface when the verb mounts the full Geppetto sections.

## Step 3: Build Cobra Through Glazed

This section explains the point where `--print-parsed-fields` starts working.

Once the command implements `cmds.BareCommand` or `cmds.WriterCommand`, build the Cobra command through Glazed:

```go
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
```

This is what enables:

- `--print-parsed-fields`
- `--print-schema`
- `--print-yaml`
- normal Glazed command settings parsing

If the verb still bypasses `cli.BuildCobraCommand(...)`, it will keep behaving like a special-case Cobra command.

## Step 4: Use a Pinocchio-Aware Config Middleware

This section covers the most common migration mistake.

Pinocchio config files are not a pure Glazed section map. They contain extra top-level keys like `repositories`. If you use the default config loader directly, those keys can break parsing.

The `pinocchio js` fix was to use `profilebootstrap.MapPinocchioConfigFile(...)` inside the middleware chain:

```go
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
```

Why this matters:

- `ResolveCLIConfigFiles(...)` respects `--config-file`,
- `MapPinocchioConfigFile(...)` ignores non-section keys like `repositories`,
- and config-derived fields show up in the parsed-field log with file metadata.

## Step 5: Resolve Final Engine Settings Once

This section is the core simplification. Do not reimplement profile merging inside the verb.

After parsing, call:

```go
resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsed)
if err != nil {
	return err
}
if resolved.Close != nil {
	defer resolved.Close()
}
```

The result gives you:

- `resolved.ProfileSelection`
- `resolved.BaseInferenceSettings`
- `resolved.FinalInferenceSettings`
- `resolved.ResolvedEngineProfile`

For a normal engine-backed verb, the standard path is:

```go
engineInstance, err := enginefactory.NewEngineFromSettings(resolved.FinalInferenceSettings)
```

For a JS runtime verb, pass `resolved.FinalInferenceSettings` into the runtime as the default engine settings so later JS-created engines inherit the resolved config/profile state.

## Step 6: Add the Debug/Provenance Exits

This section explains how to make future debugging cheap.

Once the verb resolves engine settings, support these exits:

- `--print-parsed-fields`
- `--print-inference-settings`
- `--print-inference-settings-sources`

The `pinocchio js` pattern is:

```go
debugSettings := &cmdlayers.HelpersSettings{}
if err := parsed.DecodeSectionInto(cmdlayers.GeppettoHelpersSlug, debugSettings); err != nil {
	return err
}

if debugSettings.PrintInferenceSources {
	trace, err := profilebootstrap.BuildInferenceSettingsSourceTrace(nil, parsed, resolved)
	if err != nil {
		return err
	}
	return yaml.NewEncoder(w).Encode(trace)
}

if debugSettings.PrintInferenceSettings {
	return yaml.NewEncoder(w).Encode(resolved.FinalInferenceSettings)
}
```

Use a non-nil command baseline if your verb injects extra app-specific base settings before profile overlay. Use `nil` if the Geppetto sections and config are the whole baseline.

## Step 7: If the Verb Exposes JS Profile APIs, Pass the Defaults Through

This section explains the subtle runtime bug that affected `pinocchio js`.

`gp.engines.fromResolvedProfile(...)` used to build engines from raw profile data only. That meant config-derived values like `openai-api-key` and `openai-base-url` could disappear even though the CLI had already resolved them.

The fix was:

- pass the CLI’s final merged settings into the JS runtime as default inference settings,
- and merge those defaults with the resolved profile before engine creation.

The rule to keep is simple:

- **build engines from the final merged settings, not from raw profile payload alone.**

## Minimal Template

This section gives you a compact recipe for a new verb.

```go
type MyCommand struct {
	*cmds.CommandDescription
}

func newMyCommand() (*MyCommand, error) {
	sections, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		return nil, err
	}
	debugSection, err := cmdlayers.NewInferenceDebugParameterLayer()
	if err != nil {
		return nil, err
	}

	options := []cmds.CommandDescriptionOption{
		cmds.WithShort("..."),
		cmds.WithFlags(
			fields.New("my-flag", fields.TypeString),
		),
		cmds.WithSections(debugSection),
	}
	options = append(options, cmds.WithSections(sections...))

	return &MyCommand{
		CommandDescription: cmds.NewCommandDescription("my-verb", options...),
	}, nil
}

func NewMyCommand() *cobra.Command {
	command, err := newMyCommand()
	if err != nil {
		panic(err)
	}
	cobraCommand, err := cli.BuildCobraCommand(command, cli.WithParserConfig(cli.CobraParserConfig{
		MiddlewaresFunc: myMiddlewares,
	}))
	if err != nil {
		panic(err)
	}
	return cobraCommand
}

func (c *MyCommand) RunIntoWriter(ctx context.Context, parsed *values.Values, w io.Writer) error {
	resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsed)
	if err != nil {
		return err
	}
	if resolved.Close != nil {
		defer resolved.Close()
	}

	engineInstance, err := enginefactory.NewEngineFromSettings(resolved.FinalInferenceSettings)
	if err != nil {
		return err
	}

	_ = engineInstance
	return nil
}
```

## Validation Checklist

Run these after the migration:

1. `go run ./cmd/pinocchio <verb> --help --long-help`
2. `go run ./cmd/pinocchio <verb> --print-parsed-fields`
3. `go run ./cmd/pinocchio <verb> --print-inference-settings`
4. `go run ./cmd/pinocchio <verb> --print-inference-settings-sources`
5. `go test ./cmd/pinocchio/... -count=1`
6. `go test ./pkg/cmds/profilebootstrap -count=1`

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `unknown flag: --print-parsed-fields` | The verb still bypasses `cli.BuildCobraCommand(...)` | Convert it to a Glazed command and build Cobra through `cli.BuildCobraCommand(...)` |
| `expected map[string]interface{} for section repositories` | The default config middleware is trying to parse non-section keys from `config.yaml` | Use `profilebootstrap.MapPinocchioConfigFile(...)` when loading config files |
| `must be configured when profile-settings.profile is set` | A profile was selected without any registry sources | Mount `profile-settings` and configure `profile-registries` via config or flags |
| Final engine misses API key or base URL | The verb built from raw profile data instead of merged settings | Create engines from `resolved.FinalInferenceSettings` |
| The provenance output is too thin | The verb mounted only local flags and not the Geppetto sections | Mount `CreateGeppettoSections()` and parse config/env/defaults through the Pinocchio-aware middleware chain |

## See Also

- `pinocchio/cmd/pinocchio/cmds/js.go`
- `pinocchio/pkg/cmds/profilebootstrap`
- `pinocchio/pkg/cmds/cmdlayers/helpers.go`
- `pinocchio/pkg/doc/topics/13-js-api-reference.md`
- `pinocchio/pkg/doc/topics/14-js-api-user-guide.md`
