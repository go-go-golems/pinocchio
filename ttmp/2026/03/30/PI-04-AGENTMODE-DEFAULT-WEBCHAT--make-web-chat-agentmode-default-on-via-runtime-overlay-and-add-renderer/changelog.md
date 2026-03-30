# Changelog

## 2026-03-30

- Initial workspace created
- Added a detailed design/implementation guide covering default-on runtime overlay, per-profile enabled:false opt-out, dead-flag removal, and renderer registration
- Added investigation notes summarizing the current chat endpoint merge path, default profile registry source, and frontend renderer gap
- Expanded the task list into concrete backend, frontend, testing, and documentation steps
- Implemented backend default-on runtime policy in commit `9d30b0d` (`Make web-chat agentmode default-on`)
- Removed the dead `--enable-agentmode` flag and made no-profile web-chat requests resolve to a default runtime with `agentmode`
- Added stack-aware app runtime merging using `profile.stack.lineage`, with middleware merge by `(name,id)`, ordered unique tool merge, and profile override semantics for `enabled: false` and `sanitize_yaml`
- Added a dedicated `agent_mode` renderer and frontend coverage in commit `0da28f9` (`Add web-chat agent mode renderer`)
- Verified targeted frontend checks (`vitest`, `tsc`, `vite build`) and repository pre-commit hooks
