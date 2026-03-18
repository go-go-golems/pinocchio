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

- hidden base `StepSettings` come from Pinocchio config, env, and defaults
- profile registries come from `--profile-registries`, `PINOCCHIO_PROFILE_REGISTRIES`, or the default config path
- profile selection comes from `--profile`, `PINOCCHIO_PROFILE`, config, or the registry default profile
- `--config-file` can supply the same `profile-settings.*` values used by the rest of the CLI
- scripts can call `pinocchio.engines.fromDefaults()` instead of reconstructing provider config manually

## Files

- `runner-profile-demo.js`
  - real profile-driven inference example
  - demonstrates `gp.runner.resolveRuntime(...)`
  - demonstrates `pinocchio.engines.fromDefaults(...)`
  - demonstrates a real `gp.runner.run(...)`
  - uses explicit `model` and `apiType` overrides so it does not depend on base config already defining a provider
- `runner-profile-smoke.js`
  - deterministic local smoke script used by tests
  - keeps the profile-resolution path easy to validate without calling a live model
- `profiles/basic.yaml`
  - small local profile registry used by the demo

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
