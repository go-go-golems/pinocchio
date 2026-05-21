# Tasks

## Done

- [x] Create docmgr ticket for TUI chatapp turn persistence.
- [x] Write intern-ready analysis/design/implementation guide.
- [x] Record implementation diary step for the design work.
- [x] Relate key source files and update changelog.
- [x] Upload design bundle to reMarkable.
- [x] Phase 1 implementation: persist command TUI final turns to `--turns-db` / `--turns-dsn`.
- [x] Phase 2 implementation: wire `--timeline-db` / `--timeline-dsn` to a `sessionstream` SQLite hydration store.
- [x] Add user-facing CLI help that distinguishes turns DB, timeline DB, debug JSONL, and resume behavior.
- [x] Run unit/integration tests and real tmux smoke tests after implementation.
- [x] Phase 3 implementation: add minimal `--session-id ID --resume` UX, with `convID=sessionID=--session-id`.

## TODO

- [ ] If needed, add sessionstream hydration inspection tooling for `--timeline-db` files after minimal resume works.
