# Changelog

## 2026-03-29

- Initial workspace created
- Added a detailed intern-facing design doc covering agentmode, structuredsink, sanitize-based YAML repair, and web-chat SEM integration
- Added an investigation diary with commands run, code paths inspected, documentation found, and design rationale
- Expanded the ticket task list into implementation-sized steps covering the shared parser, structuredsink wiring, web-chat integration, tests, commits, and diary updates
- Clarified in the design package that middleware and structuredsink must share one sanitize-backed parser with `sanitize_yaml` optional and defaulting to `true`
- Implemented the shared agentmode protocol/parser refactor and migrated middleware final parsing to the sanitize-backed shared path (commit `af71a50`)
- Implemented the agentmode structuredsink extractor, web-chat sink wrapper, `sanitize_yaml` runtime plumbing, and reduced noisy SEM ingress logging from debug to trace (commit `ec658b5`)
- Verified with focused package tests and repo pre-commit hooks, including `go test ./pkg/middlewares/agentmode -count=1` and `go test ./cmd/web-chat ./pkg/webchat ./pkg/middlewares/agentmode -count=1`

## 2026-03-29

Completed architecture investigation and wrote the intern-facing design package for agentmode XML-plus-YAML protocol migration, structuredsink adoption, sanitize integration, and web-chat SEM wiring.

### Related Files

- /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/events/structuredsink/filtering_sink.go — Streaming design anchored here
- /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/middlewares/agentmode/middleware.go — Current implementation analyzed
- /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/webchat/router.go — Web-chat integration seam analyzed
- /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/sanitize/pkg/yaml/sanitize.go — Sanitize-based parse plan anchored here

Validated the ticket with `docmgr doctor --root pinocchio/ttmp --ticket PI-03-AGENTMODE-STRUCTUREDSINK --stale-after 30`, added missing topic vocabulary for `structured-sinks` and `webchat`, and uploaded the bundle to reMarkable at `/ai/2026/03/29/PI-03-AGENTMODE-STRUCTUREDSINK`.
