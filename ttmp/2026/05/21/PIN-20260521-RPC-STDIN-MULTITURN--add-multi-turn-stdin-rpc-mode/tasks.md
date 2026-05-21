# Tasks

## Done

- [x] Create docmgr ticket `PIN-20260521-RPC-STDIN-MULTITURN`.
- [x] Write detailed intern-ready analysis/design/implementation guide.
- [x] Record implementation diary Step 1.
- [x] Relate key source files and update changelog.
- [x] Upload design bundle to reMarkable.

## TODO

- [ ] Phase 1: add protobuf stdin request contract (`RpcRequestLine`, submit/cancel/snapshot/shutdown).
- [ ] Phase 2: add request-id-aware JSONL writer/fanout support.
- [ ] Phase 3: add explicit `--stdin-rpc` flag and run mode without changing existing one-shot `--rpc` / `--output jsonl`.
- [ ] Phase 4: implement stdin RPC server with server-held per-session final-turn accumulators.
- [ ] Phase 5: add unit/integration tests for multi-turn context, session isolation, malformed input, shutdown, and optional cancel.
- [ ] Phase 6: run real subprocess smoke tests with a cheap profile.
- [ ] Update user-facing help once implementation lands.
