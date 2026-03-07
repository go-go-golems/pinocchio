# Changelog

## 2026-03-07

- Initial workspace created
- Added a focused design brief describing how to separate Glazed values parsing from Pinocchio webchat router construction while preserving app-owned `/chat` semantics and generic SEM/websocket infrastructure
- Added an implementation diary and expanded the ticket into a concrete execution backlog covering stream backend, router, server, tests, and migration docs
- Implemented explicit webchat constructor layers with `NewStreamBackend(...)`, `BuildRouterDepsFromValues(...)`, `NewRouterFromDeps(...)`, and `NewServerFromDeps(...)`, while keeping parsed-values wrappers in place
- Added webchat tests covering dependency-injected and parsed-values construction paths
- Added and linked a `pkg/doc` migration guide plus updates to the main webchat framework and user guides
- Verified the refactor with `go test ./pkg/webchat/...`, `go test ./pkg/doc ./cmd/web-chat ./cmd/pinocchio`, and `docmgr doctor --root pinocchio/ttmp --ticket GP-029 --stale-after 30`
- Extended GP-029 with a follow-up implementation slice for `cmd/web-chat`: remove direct AI runtime flags, use profile-registry-driven runtime composition only, and keep command parsing focused on server/profile settings
- Made `cmd/web-chat` profile-driven end to end: removed direct AI runtime flags from the command, switched runtime composition to `settings.NewStepSettings()` plus profile patches, added regression tests, and updated the web-chat docs to explain the new contract (`d4286ed`)
