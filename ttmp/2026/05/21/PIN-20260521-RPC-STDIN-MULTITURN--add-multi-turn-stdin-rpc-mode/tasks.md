# Tasks

## Done

- [x] Create docmgr ticket `PIN-20260521-RPC-STDIN-MULTITURN`.
- [x] Write detailed intern-ready analysis/design/implementation guide.
- [x] Record implementation diary Step 1.
- [x] Relate key source files and update changelog.
- [x] Upload design bundle to reMarkable.

## TODO

- [x] Phase 1: add protobuf stdin request contract (`RpcRequestLine`, submit/cancel/snapshot/shutdown).
- [x] Phase 2: add request-id-aware JSONL writer/fanout support.
- [x] Phase 3: add explicit `--stdin-rpc` flag and run mode without changing existing one-shot `--rpc` / `--output jsonl`.
- [x] Phase 4: implement stdin RPC server with server-held per-session final-turn accumulators.
- [x] Phase 5: add unit/integration tests for multi-turn context, malformed input, shutdown, and request IDs.
- [ ] Phase 5b: add stronger cancel-while-running tests.
- [x] Phase 5c: add stronger session isolation tests.
- [x] Phase 6: run real subprocess smoke tests with a cheap profile.
- [x] Update user-facing help once implementation lands.
