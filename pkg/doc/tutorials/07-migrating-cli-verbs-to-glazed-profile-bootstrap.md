---
Title: "Pinocchio Companion: Migrating CLI Verbs to the Geppetto Bootstrap Path"
Slug: cli-profile-bootstrap-migration
Short: Pinocchio-specific companion to the generic Geppetto CLI bootstrap migration tutorial.
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
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

The generic migration guide now lives in Geppetto at `geppetto/pkg/doc/tutorials/09-migrating-cli-commands-to-glazed-bootstrap-profile-resolution.md`. Read that first.
## Conceptual Companion

If you are new to the settings lifecycle, read `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md` alongside this tutorial.

That companion topic explains:

- hidden base settings
- stripped parsed-values base settings
- `BaseInferenceSettings` vs `FinalInferenceSettings`
- runtime profile switching without losing the baseline

- `ai-client`

It also means cross-profile client settings such as `ai-client.*` are part of the same shared baseline surface when the verb mounts the full Geppetto sections.

This companion page covers only the Pinocchio-specific deltas you still need when the target command lives in the Pinocchio repo.

## What Is Pinocchio-Specific

Pinocchio should now contribute only application wiring:

- its bootstrap config wrapper, `profilebootstrap.BootstrapConfig()`
- its config-file mapper, `profilebootstrap.MapPinocchioConfigFile(...)`
- its convenience wrappers like `profilebootstrap.ResolveCLIEngineSettings(...)`
- any Pinocchio runtime details, such as the JS runtime default settings path

The generic bootstrap and inference-debug behavior is Geppetto-owned.

## Step 1: Use the Pinocchio Bootstrap Config

When a Pinocchio command needs to call Geppetto bootstrap helpers directly, pass:

- `profilebootstrap.BootstrapConfig()`

That wraps the Pinocchio app name, env prefix, config-file mapper, and shared section builders without re-exporting any debug helper behavior.

## Step 2: Keep the Pinocchio Config Mapper in the Middleware Chain

Pinocchio config files contain top-level keys like `repositories`, so the middleware chain still needs:

- `profilebootstrap.MapPinocchioConfigFile(...)`

If you omit that mapper, config parsing can fail or silently drift.

## Step 3: Resolve Final Settings Through the Pinocchio Wrapper

In Pinocchio commands, the simplest path is still:

```go
resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsed)
if err != nil {
	return err
}
if resolved.Close != nil {
	defer resolved.Close()
}
```

That keeps the command code concise while still using the Geppetto-owned bootstrap system under the hood.

## Step 4: Use the Geppetto Debug Helper Directly

Pinocchio should no longer define its own debug helper path. The command should mount:

- `geppettobootstrap.NewInferenceDebugSection()`

and should execute:

- `geppettobootstrap.HandleInferenceDebugOutput(...)`

The current debug surface is one flag:

- `--print-inference-settings`

That output now includes both `settings` and `sources`, with sensitive values masked as `***`.

## Step 5: Preserve the JS Runtime Defaults Rule

The Pinocchio JS command still has one subtle runtime rule that matters:

- pass the resolved final inference settings into the JS runtime as defaults

That prevents later JS-created engines from dropping config-derived values like API keys or base URLs.

## Validation Checklist

Run these after migrating a Pinocchio verb:

1. `go run ./cmd/pinocchio <verb> --help --long-help`
2. `go run ./cmd/pinocchio <verb> --print-parsed-fields`
3. `go run ./cmd/pinocchio <verb> --print-inference-settings`
4. Confirm the debug output includes both `settings:` and `sources:`
5. `go test ./cmd/pinocchio/... -count=1`
6. `go test ./pkg/cmds/profilebootstrap -count=1`

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| Config parsing chokes on `repositories` | The command skipped the Pinocchio config-file mapper | Use `profilebootstrap.MapPinocchioConfigFile(...)` |
| The command still has `--print-inference-settings-sources` docs or flags | The migration copied older Pinocchio patterns | Replace them with `geppettobootstrap.NewInferenceDebugSection()` and the single `--print-inference-settings` path |
| JS-created engines miss API keys or base URLs | The runtime used raw profile data instead of merged defaults | Pass `resolved.FinalInferenceSettings` into the JS runtime as default settings |

## See Also

- `geppetto/pkg/doc/tutorials/09-migrating-cli-commands-to-glazed-bootstrap-profile-resolution.md`
- [cmd/pinocchio/cmds/js.go](../../../cmd/pinocchio/cmds/js.go)
- [pkg/cmds/profilebootstrap/profile_selection.go](../../cmds/profilebootstrap/profile_selection.go)
