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
- `--config-file` can supply the same unified `profile.*` values used by the rest of the CLI
- scripts resolve settings with `gp.inferenceProfiles.resolve()` and execute through `gp.agent().session().next().run()`
- `--turns-dsn` / `--turns-db` install a default `gp.turnStores.default()` store for session persistence and resume

If your default `profiles.yaml` is still in the old mixed-runtime format, rewrite it first to the engine-only `inference_settings` shape. Use [profiles/basic.yaml](./profiles/basic.yaml) as the reference.

## Files

- `runner-profile-demo.js`
  - real profile-driven inference example
  - demonstrates `gp.inferenceProfiles.resolve()`
  - demonstrates session-centered `session.next().run()` execution
- `runner-profile-smoke.js`
  - deterministic local smoke script used by tests
  - proves that profile selection resolves engine settings and builds a session-capable agent without calling a live model
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

Enable durable turn persistence for a script with:

```bash
pinocchio js \
  --script path/to/session-script.js \
  --profile-registries "$HOME/.config/pinocchio/profiles.yaml" \
  --turns-db /tmp/pinocchio-js-turns.db
```
