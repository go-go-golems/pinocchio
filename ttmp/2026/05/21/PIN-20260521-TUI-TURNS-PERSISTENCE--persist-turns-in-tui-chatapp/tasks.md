# Tasks

## Done

- [x] Create docmgr ticket for TUI chatapp turn persistence.
- [x] Write intern-ready analysis/design/implementation guide.
- [x] Record implementation diary step for the design work.
- [x] Relate key source files and update changelog.
- [x] Upload design bundle to reMarkable.

## TODO

- [ ] Phase 1 implementation: persist command TUI final turns to `--turns-db` / `--turns-dsn`.
- [ ] Phase 2 implementation: wire `--timeline-db` / `--timeline-dsn` to a `sessionstream` SQLite hydration store.
- [ ] Phase 3 design/implementation: add stable session/conversation id and resume UX.
- [ ] Add user-facing CLI help that distinguishes turns DB, timeline DB, debug JSONL, and resume behavior.
- [ ] Run unit/integration tests and real tmux smoke tests after implementation.
