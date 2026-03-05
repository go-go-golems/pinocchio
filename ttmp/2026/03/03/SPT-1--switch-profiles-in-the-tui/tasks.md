# Tasks

## TODO

- [x] Implement submit interception hook in Bobatea chat model (so `/profile` doesn’t hit inference)
- [x] Implement header hook in Bobatea chat model (display current profile/runtime key)
- [x] Add new runnable TUI command that uses real inference + profile registry selection
- [x] Cobra flags: `--profile-registries` (required), `--profile` (optional), `--conv-id` (optional)
- [x] Fail startup if registry stack loads zero profiles
- [x] Use `/tmp/profile-registry.yaml` for smoke testing (switch mento-haiku-4.5 ↔ mento-sonnet-4.6)
- [x] Implement `/profile` slash command
- [x] `/profile` opens modal picker
- [x] `/profile <slug>` switches directly
- [x] show errors in timeline if slug not found / cannot switch while streaming
- [x] Implement `ProfileManager` (load registry stack, list profiles, resolve effective runtime)
- [x] Implement profile-aware backend
- [x] keep one `session.Session` with turn history
- [x] swap `session.Builder` when profile changes (idle-only)
- [x] set `turns.KeyTurnMetaRuntime` on each new turn based on current profile selection
- [x] Persistence (must verify)
- [x] Turn persistence: store final turns into `SQLiteTurnStore` with `runtime_key` column populated
- [x] Timeline persistence: store assistant messages into `SQLiteTimelineStore` with runtime/profile attribution in props
- [x] Store explicit “profile switched” markers in timeline persistence
- [x] Add scripts in repo `scripts/` (per user instruction)
- [x] tmux smoke script to run TUI and auto-send keystrokes (real inference)
- [x] persistence verification script to query sqlite DBs and assert runtime boundaries
- [x] Add/extend tests (unit-level where feasible)
- [x] ProfileManager: parse/list/resolve
- [x] Timeline persister: persists runtime/profile props when event metadata contains them
- [x] Update diary continuously while implementing
- [x] Write exhaustive intern postmortem and upload bundle to reMarkable

## Done

<!-- Move items here as they are completed -->
