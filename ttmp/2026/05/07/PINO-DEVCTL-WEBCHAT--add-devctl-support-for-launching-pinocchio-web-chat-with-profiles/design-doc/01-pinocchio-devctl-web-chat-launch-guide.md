---
Title: Pinocchio devctl Web Chat Launch Guide
Ticket: PINO-DEVCTL-WEBCHAT
Status: active
Topics:
    - devctl
    - web-chat
    - profiles
    - developer-experience
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: .devctl.yaml
      Note: Root devctl profile and plugin entry point
    - Path: cmd/web-chat/.devctl.yaml
      Note: Subdirectory devctl profile and plugin entry point
    - Path: cmd/web-chat/main.go
      Note: web-chat CLI flags and server startup
    - Path: cmd/web-chat/plugins/webchat.py
      Note: devctl protocol plugin implementation
    - Path: cmd/web-chat/profiles/api.go
      Note: web-chat profile HTTP API used for health checks
    - Path: pkg/cmds/profilebootstrap/profile_selection.go
      Note: profile settings bootstrap and registry resolution
ExternalSources: []
Summary: Intern-facing architecture, design, and implementation guide for launching Pinocchio web-chat with devctl profiles.
LastUpdated: 2026-05-07T17:25:00-04:00
WhatFor: Explain how devctl launches Pinocchio web-chat, how devctl profiles map to web-chat profile-aware runtime selection, how the plugin protocol works, and how to validate or extend the integration.
WhenToUse: Use when onboarding an intern to the devctl web-chat launcher, reviewing the implementation, debugging local launch failures, or extending launch profiles.
---


# Pinocchio devctl Web Chat Launch Guide

## Executive Summary

Pinocchio's `web-chat` command is a concrete application that combines several subsystems:

- a Go HTTP/WebSocket backend in `cmd/web-chat`;
- the reusable chat runtime under `pkg/chatapp`;
- the web frontend under `cmd/web-chat/web`;
- profile-aware runtime selection from Pinocchio and Geppetto profile registries;
- optional debug APIs and Geppetto provider observability;
- durable SQLite timeline and turn stores for local development.

This ticket adds first-class `devctl` support for launching that app from the Pinocchio repository root. A new contributor can now run:

```bash
cd pinocchio
devctl profiles list
devctl plan
devctl up
```

`devctl` handles process supervision, logs, health checks, restart behavior, and shutdown. The repo-local plugin knows the Pinocchio-specific details: where the web-chat app lives, how to build the Go binary, how to run the Vite frontend, where to place local SQLite files, and which web-chat profile flags should be passed to the backend.

The most important design point is the two-layer meaning of “profiles”:

1. **devctl profiles** select the local launch mode. For example, `web-chat` runs normally, while `web-chat-observe` enables provider-level observability.
2. **Pinocchio / Geppetto profile settings** select the AI runtime profile used by chat sessions. The devctl plugin can pass `--profile` and `--profile-registries` into `web-chat`, but it can also leave them empty so `web-chat` uses the normal Pinocchio config-resolution chain.

The implementation is intentionally conservative: one repo-local Python plugin emits protocol-clean NDJSON, returns service definitions, and lets `devctl` supervise both backend and Vite. No long-running service is started inside the plugin itself.

## Problem Statement

Before this change, local web-chat startup required knowing several scattered facts:

- The Go app lives under `cmd/web-chat`, not at the repo root.
- The Vite app lives under `cmd/web-chat/web`.
- The backend should be built from the module root with `go build -o cmd/web-chat/bin/web-chat ./cmd/web-chat`.
- The runtime should use local SQLite files for timeline and turns:
  - `cmd/web-chat/var/devctl/timeline.sqlite`
  - `cmd/web-chat/var/devctl/turns.sqlite`
- The frontend dev server needs `VITE_BACKEND_ORIGIN` so browser API and WebSocket traffic can reach the Go backend.
- The new profile-aware web-chat runtime can be configured via flags:
  - `--profile`
  - `--profile-registries`
- Debug and observability modes require additional flags:
  - `--debug-api`
  - `--geppetto-trace-level provider`

An intern should not have to reconstruct these details from prior shell history or previous debugging sessions. The launch contract belongs in versioned repo files and a clear guide.

## Proposed Solution

Add and maintain a `devctl` plugin that launches Pinocchio web-chat as two supervised services:

1. `backend`: the compiled Go `web-chat` binary.
2. `vite`: the frontend dev server.

The plugin implements the devctl protocol v2 operations:

- `config.mutate`
- `validate.run`
- `build.run`
- `prepare.run`
- `launch.plan`
- `command.run`

The repository has two devctl config entry points:

- `.devctl.yaml` at the Pinocchio root for normal developer use.
- `cmd/web-chat/.devctl.yaml` for the app subdirectory workflow.

Both configs select the same plugin ID, `pinocchio-webchat`, and expose the same launch profiles.

## High-Level Architecture

```text
Developer
  |
  | devctl up [--profile web-chat-observe]
  v
devctl CLI
  |
  | reads .devctl.yaml
  | selects active profile
  | starts plugin as subprocess
  v
cmd/web-chat/plugins/webchat.py
  |
  | config.mutate -> ports, URLs, profile/trace settings
  | validate.run  -> tools and node_modules checks
  | build.run     -> Go backend binary
  | prepare.run   -> pnpm install when needed
  | launch.plan   -> backend + Vite service specs
  v
devctl supervisor
  |
  | starts and tracks services
  | captures logs under .devctl/logs
  | runs health checks
  v
+----------------------+       +-----------------------+
| backend              |       | vite                  |
| web-chat Go binary   |<------| frontend dev server   |
| :8092 by default     | API   | :5174 by default      |
+----------------------+  WS   +-----------------------+
```

The plugin only plans and builds. `devctl` owns process lifecycle:

- start;
- status;
- logs;
- restart;
- service stop/start;
- shutdown with `devctl down`.

## File Reference Map

### devctl files

| File | Purpose |
| --- | --- |
| `.devctl.yaml` | Root-level devctl entry point for launching web-chat from `pinocchio/`. |
| `cmd/web-chat/.devctl.yaml` | Subdirectory devctl entry point for launching from `pinocchio/cmd/web-chat/`. |
| `cmd/web-chat/plugins/webchat.py` | Protocol v2 plugin that builds, validates, prepares, and plans web-chat services. |
| `.gitignore` | Ignores root `.devctl/` state/logs generated by devctl. |

### web-chat backend files

| File | Purpose |
| --- | --- |
| `cmd/web-chat/main.go` | Cobra/Glazed command wiring for `web-chat web-chat`; defines flags and starts the HTTP server. |
| `cmd/web-chat/runtime_composer.go` | Builds profile-driven runtime composition for inference. |
| `cmd/web-chat/profiles/api.go` | HTTP profile API routes such as `/api/chat/profiles` and `/api/chat/profile`. |
| `cmd/web-chat/profiles/resolver.go` | Resolves selected profile and registry into runtime plans. |
| `pkg/cmds/profilebootstrap/profile_selection.go` | CLI/config bootstrap layer for `.pinocchio.yml`, `--profile`, and `--profile-registries`. |
| `pkg/configdoc/types.go` | Defines `.pinocchio.yml` / `.pinocchio.override.yml` profile config document shape. |

### frontend files

| File | Purpose |
| --- | --- |
| `cmd/web-chat/web/package.json` | Frontend scripts and dependencies. |
| `cmd/web-chat/web/vite.config.ts` | Vite dev-server behavior. |
| `cmd/web-chat/web/src/config/runtimeConfig.ts` | Browser runtime config discovery. |
| `cmd/web-chat/web/src/ws/wsManager.ts` | WebSocket lifecycle and hydration coordinator. |

## The Two Profile Systems

### 1. devctl profiles: launch-mode profiles

`devctl` profiles are defined in `.devctl.yaml`:

```yaml
profile:
  active: web-chat

profiles:
  web-chat:
    plugins:
      - pinocchio-webchat
    env:
      PINOCCHIO_WEBCHAT_DEBUG_API: "true"
      PINOCCHIO_WEBCHAT_TRACE_LEVEL: "off"

  web-chat-observe:
    plugins:
      - pinocchio-webchat
    env:
      PINOCCHIO_WEBCHAT_DEBUG_API: "true"
      PINOCCHIO_WEBCHAT_TRACE_LEVEL: "provider"
```

These profiles answer: **How should the local development environment run?**

- `web-chat` is the default local development mode.
- `web-chat-observe` enables provider observability for debugging provider-to-browser event correlation.

Select a mode with:

```bash
devctl up --profile web-chat-observe
```

or make a local default in `.devctl.override.yaml`:

```yaml
profile:
  active: web-chat-observe
```

Do not commit `.devctl.override.yaml` unless the team intentionally wants shared local overrides.

### 2. Pinocchio / Geppetto profiles: AI runtime profiles

Pinocchio profile settings answer: **Which AI runtime configuration should a chat session use?**

The `web-chat` command supports:

```bash
--profile <slug>
--profile-registries <source1,source2,...>
```

Those flags flow through:

```text
web-chat CLI flags
  -> profilebootstrap.ResolveCLIProfileRuntime
  -> configdoc + Geppetto engine profile registry chain
  -> profiles.RequestResolver
  -> runtime composer
  -> chat session runtime
```

The devctl plugin exposes this through environment variables:

| Env var | Meaning |
| --- | --- |
| `PINOCCHIO_WEBCHAT_PROFILE` | If set, passed as `--profile`. |
| `PINOCCHIO_WEBCHAT_PROFILE_REGISTRIES` | If set, passed as `--profile-registries`. |
| `PINOCCHIO_WEBCHAT_TRACE_LEVEL` | Passed as `--geppetto-trace-level`; default `off`. |
| `PINOCCHIO_WEBCHAT_DEBUG_API` | Enables/disables `--debug-api`; default true. |
| `PINOCCHIO_WEBCHAT_ROOT` | Passed as `--root`; default `/`. |
| `PINOCCHIO_WEBCHAT_BACKEND_PORT` | Preferred backend port; default 8092. |
| `PINOCCHIO_WEBCHAT_VITE_PORT` | Preferred Vite port; default 5174. |

Example local override that pins a specific AI profile:

```yaml
profiles:
  web-chat:
    env:
      PINOCCHIO_WEBCHAT_PROFILE: default
      PINOCCHIO_WEBCHAT_PROFILE_REGISTRIES: /home/me/.config/pinocchio/profiles.yaml
```

## Plugin Protocol Overview

The plugin in `cmd/web-chat/plugins/webchat.py` is a devctl protocol v2 plugin. The first line on stdout is the handshake:

```json
{
  "type": "handshake",
  "protocol_version": "v2",
  "plugin_name": "pinocchio-webchat",
  "capabilities": {
    "ops": [
      "config.mutate",
      "validate.run",
      "build.run",
      "prepare.run",
      "launch.plan",
      "command.run"
    ]
  }
}
```

After the handshake, stdout is reserved for NDJSON protocol frames only. Human-readable logs go to stderr through the `log()` helper.

### Pseudocode: plugin loop

```python
emit_handshake()

for line in stdin:
    request = json.loads(line)
    op = request["op"]
    ctx = request["ctx"]

    if op == "config.mutate":
        compute_ports_and_profile_settings()
        emit_config_patch()

    elif op == "validate.run":
        check_go_node_npx_pnpm_node_modules()
        emit_valid_or_errors()

    elif op == "build.run":
        if not dry_run:
            go_build_web_chat_binary()
        emit_step_results()

    elif op == "prepare.run":
        if node_modules_missing and not dry_run:
            pnpm_install()
        emit_step_results()

    elif op == "launch.plan":
        emit_backend_and_vite_service_specs()

    elif op == "command.run":
        run_named_helper_command()

    else:
        emit_E_UNSUPPORTED()
```

## Operation Details

### `config.mutate`

`config.mutate` computes runtime facts:

- backend port;
- Vite port;
- backend URL;
- frontend URL;
- trace level;
- debug API setting;
- optional profile and profile registries.

The plugin prefers default ports but falls back to a free port if the preferred one is occupied.

Default config patch shape:

```json
{
  "services": {
    "backend": {
      "port": 8092,
      "url": "http://127.0.0.1:8092"
    },
    "vite": {
      "port": 5174,
      "url": "http://127.0.0.1:5174"
    }
  },
  "webchat": {
    "profile": "",
    "profile_registries": "",
    "trace_level": "off",
    "debug_api": true,
    "root": "/"
  },
  "env": {
    "VITE_BACKEND_ORIGIN": "http://127.0.0.1:8092"
  }
}
```

### `validate.run`

Validation checks:

- `go` exists;
- `node` exists;
- `npx` exists;
- frontend `node_modules` exists;
- `pnpm` exists if `node_modules` is missing;
- `go.mod` exists at or above the web-chat root.

Validation returns actionable messages. Example:

```json
{
  "valid": false,
  "errors": [
    {
      "key": "frontend.node_modules",
      "message": "node_modules missing: run 'cd cmd/web-chat/web && pnpm install'"
    }
  ],
  "warnings": []
}
```

### `build.run`

`build.run` builds the Go backend binary:

```bash
go build -o cmd/web-chat/bin/web-chat ./cmd/web-chat
```

The output artifact is:

```text
cmd/web-chat/bin/web-chat
```

The binary is intentionally under `cmd/web-chat/bin/`, which is ignored by `cmd/web-chat/.gitignore`.

### `prepare.run`

`prepare.run` installs frontend dependencies only when needed:

```bash
cd cmd/web-chat/web
pnpm install
```

If `node_modules` already exists, the step is reported as successful with a skipped reason.

### `launch.plan`

`launch.plan` returns two services.

#### Backend service

The backend service command is roughly:

```bash
mkdir -p cmd/web-chat/var/devctl && exec cmd/web-chat/bin/web-chat \
  web-chat \
  --addr :8092 \
  --root / \
  --timeline-db cmd/web-chat/var/devctl/timeline.sqlite \
  --turns-db cmd/web-chat/var/devctl/turns.sqlite \
  --geppetto-trace-level off \
  --debug-api
```

If `PINOCCHIO_WEBCHAT_PROFILE` is set, the plugin appends:

```bash
--profile <profile>
```

If `PINOCCHIO_WEBCHAT_PROFILE_REGISTRIES` is set, the plugin appends:

```bash
--profile-registries <registries>
```

The wrapper uses:

```bash
bash --noprofile --norc -lc '<command>'
```

This avoids shell startup noise such as line-editing warnings from interactive shell plugins.

Backend health check:

```text
GET http://127.0.0.1:8092/api/chat/profiles
```

This health check verifies not only that HTTP is listening, but also that the profile API is mounted.

#### Vite service

The Vite service command is roughly:

```bash
cd cmd/web-chat/web
VITE_BACKEND_ORIGIN=http://127.0.0.1:8092 \
  npx vite --port 5174 --clearScreen false
```

Vite health check:

```text
GET http://127.0.0.1:5174/
```

## Runtime Data and Logs

### devctl state and logs

At the repository root, devctl writes:

```text
.devctl/
  state.json
  logs/
    backend-YYYYMMDD-HHMMSS.stdout.log
    backend-YYYYMMDD-HHMMSS.stderr.log
    vite-YYYYMMDD-HHMMSS.stdout.log
    vite-YYYYMMDD-HHMMSS.stderr.log
```

`.devctl/` is ignored in the root `.gitignore`.

Useful commands:

```bash
devctl status --tail-lines 20
devctl logs --service backend --stderr --follow
devctl logs --service vite --follow
devctl down
```

### web-chat local data

The backend writes local development state under:

```text
cmd/web-chat/var/devctl/
  timeline.sqlite
  turns.sqlite
```

This directory is ignored by `cmd/web-chat/.gitignore`.

## API References

### devctl commands

| Command | Purpose |
| --- | --- |
| `devctl profiles list` | List configured devctl launch profiles. |
| `devctl profiles active` | Show active profile after config/override resolution. |
| `devctl plugins list` | Verify plugin handshake and capabilities. |
| `devctl plan` | Show computed config and services without launching. |
| `devctl up` | Run config/build/prepare/validate/launch and supervise services. |
| `devctl up --profile web-chat-observe` | Launch with provider observability mode. |
| `devctl status` | Show tracked services and log paths. |
| `devctl logs --service backend --stderr --follow` | Tail backend stderr. |
| `devctl down` | Stop services and remove devctl state. |

### web-chat backend APIs used by this integration

| Endpoint | Purpose |
| --- | --- |
| `GET /api/chat/profiles` | List available Pinocchio/Geppetto engine profiles. Used as backend health check. |
| `GET /api/chat/profile` | Read current profile cookie/default selection. |
| `POST /api/chat/profile` | Set current profile cookie. |
| `POST /api/chat/sessions` | Create a chat session. |
| `GET /api/chat/ws` | Sessionstream WebSocket endpoint. |
| `GET /api/debug/sessions/{id}/geppetto` | Debug API for Geppetto observability records when debug API and tracing are enabled. |

## Validation Results

### Static/plugin validation

```bash
cd pinocchio
python3 -m py_compile cmd/web-chat/plugins/webchat.py
devctl profiles list
devctl plugins list
devctl plan --dry-run
devctl plan --profile web-chat-observe --dry-run
devctl up --dry-run --timeout 60s
```

Observed results:

- `profiles list` shows `web-chat` active and `web-chat-observe` available.
- `plugins list` shows `pinocchio-webchat` with protocol v2 and all expected ops.
- `plan --dry-run` returns two services: `backend` and `vite`.
- `web-chat-observe` changes `webchat.trace_level` to `provider` and appends `--geppetto-trace-level provider`.
- `up --dry-run` reports successful build and prepare step objects with `ok: true`.

### Real launch smoke

Command:

```bash
cd pinocchio
devctl up --force --timeout 180s
devctl status --tail-lines 5
python3 - <<'PY'
import urllib.request
with urllib.request.urlopen('http://127.0.0.1:8092/api/chat/profiles', timeout=5) as r:
    print(r.status, r.read().decode()[:300])
PY
devctl down
```

Observed result:

- `devctl up` built the backend binary.
- `backend` and `vite` started and passed health checks.
- `devctl status` showed both services alive.
- `/api/chat/profiles` returned HTTP 200 and profile JSON.
- `devctl down` stopped profile `web-chat` successfully.

## Design Decisions

### Decision 1: Keep one plugin and use profiles for modes

Using one plugin keeps launch knowledge in one place. Profiles adjust environment variables for launch modes.

Good:

- one protocol implementation;
- one validation path;
- easy to add new modes.

Example future profile:

```yaml
profiles:
  web-chat-local-profile:
    plugins: [pinocchio-webchat]
    env:
      PINOCCHIO_WEBCHAT_PROFILE: local-dev
      PINOCCHIO_WEBCHAT_PROFILE_REGISTRIES: ./profiles.local.yaml
```

### Decision 2: Support both repo root and `cmd/web-chat`

The plugin detects the application root:

```python
if repo_root contains plugins/webchat.py and web/:
    app_root = repo_root
else if repo_root contains cmd/web-chat/plugins/webchat.py:
    app_root = repo_root/cmd/web-chat
```

This preserves the old subdirectory workflow while enabling the root workflow.

### Decision 3: Let devctl supervise services

The plugin does not start long-running services. It returns a launch plan. This follows devctl's intended architecture and keeps logs/status/restart behavior consistent.

### Decision 4: Use `bash --noprofile --norc`

The service commands need a small shell wrapper for `mkdir -p` and `exec`. Using `--noprofile --norc` avoids local shell startup files contaminating logs.

### Decision 5: Keep Pinocchio profile selection optional

The plugin does not force `--profile default`. If `PINOCCHIO_WEBCHAT_PROFILE` is empty, web-chat uses Pinocchio's standard config resolution chain. This matters because developers may already have `.pinocchio.yml`, `.pinocchio.override.yml`, or user config files.

## Alternatives Considered

### Alternative: only keep `cmd/web-chat/.devctl.yaml`

Rejected because new contributors usually start from the repository root. Root support makes `devctl up` discoverable.

### Alternative: start services inside the plugin

Rejected because it would duplicate devctl's supervisor and break `devctl status`, `devctl logs`, `devctl restart`, and `devctl down` expectations.

### Alternative: hard-code one AI profile

Rejected because profile selection is user- and environment-specific. The plugin should support profile selection, but not force it.

### Alternative: separate plugins for backend and Vite

Rejected for now. The backend and Vite plans are tightly coupled through `VITE_BACKEND_ORIGIN`, ports, and health checks. A single plugin is simpler until another service needs independent ownership.

## Troubleshooting Guide

### `devctl plugins list` fails

Likely causes:

- Python syntax error in `cmd/web-chat/plugins/webchat.py`.
- The plugin printed non-JSON to stdout before the handshake.
- `.devctl.yaml` has the wrong plugin path.

Commands:

```bash
python3 -m py_compile cmd/web-chat/plugins/webchat.py
devctl --log-level debug plugins list
```

### `validate.run` says `node_modules` is missing

Run:

```bash
cd cmd/web-chat/web
pnpm install
```

Or let `devctl up` run `prepare.run` if `pnpm` is installed.

### Backend health check fails

Check backend stderr:

```bash
devctl logs --service backend --stderr --tail-lines 100
```

Common causes:

- profile registry config is invalid;
- selected `PINOCCHIO_WEBCHAT_PROFILE` does not exist;
- backend port is already occupied after planning;
- Go binary build is stale or failed.

### Vite health check fails

Check Vite logs:

```bash
devctl logs --service vite --stderr --tail-lines 100
devctl logs --service vite --tail-lines 100
```

Common causes:

- frontend dependencies missing;
- Vite port occupied;
- `npx` not on PATH.

### Want provider observability?

Use:

```bash
devctl up --profile web-chat-observe --force
```

Then inspect debug APIs after creating a chat session:

```text
GET /api/debug/sessions/{session_id}/geppetto
```

## Extension Playbook

### Add a new launch mode

1. Add a profile to `.devctl.yaml` and `cmd/web-chat/.devctl.yaml`.
2. Set env vars for the mode.
3. Run `devctl profiles list`.
4. Run `devctl plan --profile <new-profile> --dry-run`.
5. Validate that the service command contains the intended flags.

Example:

```yaml
profiles:
  web-chat-local-profile:
    display_name: Web Chat Local Profile
    plugins: [pinocchio-webchat]
    env:
      PINOCCHIO_WEBCHAT_PROFILE: local-dev
      PINOCCHIO_WEBCHAT_PROFILE_REGISTRIES: ./profiles.local.yaml
```

### Add a new backend flag

1. Add a `PINOCCHIO_WEBCHAT_*` env var reader in `webchat.py`.
2. Return it in `config.mutate` under `webchat.*`.
3. Read it from `webchat_cfg` in `launch.plan`.
4. Append the corresponding CLI flag to `backend_args`.
5. Test with `devctl plan --dry-run`.

Pseudocode:

```python
# config.mutate
my_flag = env_str("PINOCCHIO_WEBCHAT_MY_FLAG")
set["webchat.my_flag"] = my_flag

# launch.plan
my_flag = webchat_cfg.get("my_flag", "")
if my_flag:
    backend_args.extend(["--my-flag", my_flag])
```

## Implementation Plan Completed

- [x] Read devctl help pages.
- [x] Create `PINO-DEVCTL-WEBCHAT` ticket.
- [x] Add root `.devctl.yaml`.
- [x] Update `cmd/web-chat/.devctl.yaml`.
- [x] Make the plugin repo-root aware.
- [x] Add devctl profiles.
- [x] Add profile-aware web-chat CLI flag threading.
- [x] Validate plugin planning and real launch.
- [x] Write this guide.

## Open Questions

- Should the repo eventually include a shared sample `.devctl.override.yaml` for local AI profile registry paths?
- Should `web-chat-observe` also set a smaller recorder retention limit for lighter debug sessions?
- Should there be separate frontend-only or backend-only devctl profiles?

## References

- `devctl help --all`
- `devctl help user-guide`
- `devctl help scripting-guide`
- `devctl help plugin-authoring`
- `devctl help profiles-guide`
- `cmd/web-chat/main.go`
- `pkg/cmds/profilebootstrap/profile_selection.go`
- `cmd/web-chat/profiles/api.go`
