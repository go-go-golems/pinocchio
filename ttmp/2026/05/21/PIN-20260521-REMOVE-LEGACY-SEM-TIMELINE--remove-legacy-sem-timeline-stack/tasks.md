# Tasks

## Done

- [x] Create docmgr ticket for deleting the legacy `sem` timeline stack.
- [x] Audit remaining `sem` and legacy timeline references.
- [x] Write intern-ready analysis/design/implementation guide.
- [x] Record implementation diary step.
- [x] Relate key source files and update changelog.
- [x] Upload design bundle to reMarkable.
- [x] Remove `cmd/web-chat/timeline` command group and root wiring.
- [x] Remove `chatstore.TimelineStore` interface, SQLite/in-memory implementations, and tests.
- [x] Split/simplify `pkg/cmds/chat_persistence.go` so it no longer opens legacy timeline stores.
- [x] Remove `proto/sem`, generated `pkg/sem`, and generated TypeScript `src/sem` outputs after references are gone.
- [x] Update `buf.gen.yaml`, `Makefile`, `cmd/web-chat/web/biome.json`, and docs that mention `sem` timeline generation.
- [x] Audit and remove unused `cmd/web-chat/proto/sem` excluded proto island.
- [x] Validate with targeted tests, `make proto-gen`, `make schema-vet`, full Go tests, and web typecheck/lint.

## TODO

- [ ] If users need timeline inspection again, create replacement tooling against `sessionstream.HydrationStore` instead of restoring the old `sem` timeline stack.
