# Pinocchio JS Examples

These examples are meant to be run with:

```bash
pinocchio js --script <file.js>
```

The command exposes:

- `require("geppetto")`
- `require("pinocchio")`

The important difference from the Geppetto example shell is that `pinocchio js` uses Pinocchio's own config/profile bootstrap.

That means:

- hidden base `InferenceSettings` come from Pinocchio config, env, and defaults
- engine profile registries come from `--profile-registries`, `PINOCCHIO_PROFILE_REGISTRIES`, config, or the default `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml`
- engine profile selection comes from `--profile`, `PINOCCHIO_PROFILE`, config, or the registry default profile
- `--config-file` can supply the same `profile-settings.*` values used by the rest of the CLI
- scripts can either resolve an engine profile with `gp.profiles.resolve({})` and build with `gp.engines.fromResolvedProfile(...)`, or build directly from hidden base config with `pinocchio.engines.fromDefaults()`

If your default `profiles.yaml` is still in the old mixed-runtime format, rewrite it first:

```bash
go run ./scripts/migrate_engine_profiles_yaml.go --in-place
```

## Files

- `runner-profile-demo.js`
  - real profile-driven inference example
  - demonstrates `gp.profiles.resolve({})`
  - demonstrates `gp.engines.fromResolvedProfile(...)`
  - demonstrates a real `gp.runner.run(...)`
- `runner-profile-smoke.js`
  - deterministic local smoke script used by tests
  - proves that profile selection changes the resolved engine settings without calling a live model
- `profiles/basic.yaml`
  - small local engine-profile registry used by the demo

## Run

```bash
pinocchio js \
  --script examples/js/runner-profile-demo.js \
  --profile-registries examples/js/profiles/basic.yaml
```

Or pick the explicit assistant profile:

```bash
pinocchio js \
  examples/js/runner-profile-demo.js \
  --profile assistant \
  --profile-registries examples/js/profiles/basic.yaml
```

Use the smoke script if you want deterministic local output:

```bash
pinocchio js \
  --script examples/js/runner-profile-smoke.js \
  --profile-registries examples/js/profiles/basic.yaml
```
