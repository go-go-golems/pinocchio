# Tasks

## TODO

- [x] Create ticket workspace and primary docs
- [x] Investigate current `agentmode` middleware behavior and parse path
- [x] Investigate Geppetto structuredsink APIs and examples
- [x] Investigate Pinocchio web-chat sink wiring and SEM integration
- [x] Investigate `sanitize/pkg/yaml` public API for YAML repair
- [x] Write a detailed intern-facing analysis / design / implementation guide
- [x] Write an investigation diary including documentation found
- [x] Run `docmgr doctor` and resolve any doc issues
- [x] Upload the ticket bundle to reMarkable and verify the remote listing
- [x] Update the design doc to explicitly require one shared sanitize-backed parser used by both middleware and structuredsink
- [x] Expand the ticket task list into implementation-sized steps with validation checkpoints and commit boundaries
- [x] Add `agentmode` protocol constants and an XML-like prompt builder for `<pinocchio:agent_mode_switch:v1>`
- [x] Add a shared parser in `pinocchio/pkg/middlewares/agentmode` with `sanitize_yaml` optional and defaulting to `true`
- [x] Migrate middleware final parsing from `parse.ExtractYAMLBlocks` plus direct `yaml.Unmarshal` to the shared parser
- [x] Keep or add compatibility wrappers only where needed for existing tests and call sites
- [x] Add middleware-level tests for prompt generation, parsing, sanitize-on behavior, sanitize-off behavior, and final switch application
- [x] Add an `agentmode` structuredsink extractor that uses the same shared parser
- [x] Add a web-chat sink wrapper that installs the agentmode filtering sink only when agentmode middleware is enabled in the resolved runtime
- [x] Extend web-chat middleware schema/config handling to expose `sanitize_yaml`
- [x] Verify whether existing `agent.mode` SEM mapping is sufficient and only add translator changes if a new agentmode event type is introduced
- [x] Lower the noisy SEM ingress log in `pkg/webchat/sem_translator.go` from debug to trace
- [x] Add targeted tests for the web-chat sink wrapper and structured filtering behavior
- [x] Run focused `go test` coverage for `pkg/middlewares/agentmode`, `pkg/webchat`, and `cmd/web-chat`
- [x] Commit code changes in logical chunks with reviewable messages
- [x] Update the diary with implementation steps, commands, failures, validation, and commit hashes
- [x] Update the changelog and task state after each completed phase
- [x] Re-run `docmgr doctor` after implementation-facing doc updates
