# Scopedjs TUI Demo

This example will be a Bubble Tea application that demonstrates how to expose a scoped JavaScript runtime as one Geppetto tool using `geppetto/pkg/inference/tools/scopedjs`.

## Planned runtime shape

- real `fs` native module
- scoped `db` global with fake project tasks and notes
- fake `obsidian` module
- fake `webserver` module
- bootstrap helper functions such as `joinPath(...)`

## What exists in the first implementation checkpoint

- deterministic fake workspaces in `fake_data.go`
- workspace materialization into a temp directory
- `scopedjs.EnvironmentSpec` setup in `environment.go`
- direct runtime/tool smoke tests in `environment_test.go`

The Bubble Tea command wiring and custom timeline renderers will be added in later checkpoints.
