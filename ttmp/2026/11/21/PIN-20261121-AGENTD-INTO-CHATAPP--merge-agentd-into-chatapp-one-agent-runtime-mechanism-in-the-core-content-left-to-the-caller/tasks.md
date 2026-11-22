# Tasks

## TODO
- [x] Design guide written, reviewed and uploaded to reMarkable <!-- t:rzci -->
- [x] Phase 1: ComposedRuntime Loop/ToolConfig/RoundGuard passthrough <!-- t:a0rb -->
- [x] Phase 1: busy policy (reject default, replace option) with ErrRunActive <!-- t:lb9e -->
- [x] Phase 1: run outcome codes (ChatRunFinished truncated/reason, ChatRunFailed code), classifier and error-text options <!-- t:v09d -->
- [x] Phase 1: PromptRequest.Blocks appended after the user message <!-- t:5gis -->
- [x] Phase 1: chatapp saves the final turn on every outcome, closing unanswered tool calls; composers stop persisting <!-- t:0fag -->
- [x] Phase 1: remove chatapp content defaults (Explain evtstream, silent demo fallback, max-rounds warning text) <!-- t:aobr -->
- [x] Phase 2: sessions package (Info, Store, memory store) and usage plugin <!-- t:p3xi -->
- [x] Phase 3: pkg/chatapp/httpapi promoted from web-chat appserver, session routes, stop, auth hooks <!-- t:wz8o -->
- [x] Phase 3: SSE transport (snapshot then live, gap-free) and fanout multiplexer; web-chat switched <!-- t:fxol -->
- [x] Phase 4: chatapp wires browser tools; frontendtools results as any JSON value <!-- t:4wjv -->
- [x] Phase 4: toolkit: toolexec WithDeadline/CutResults, budget.SessionTokens, presets loader <!-- t:timy -->
- [x] Phase 5: pinocchio agent serve on httpapi + presets + modelsource; delete pkg/agentd <!-- t:pigi -->
- [ ] Migration playbook for homolog-bench written <!-- t:6h9p -->
- [ ] Final validation: go test ./..., make lint, docmgr doctor <!-- t:b246 -->
