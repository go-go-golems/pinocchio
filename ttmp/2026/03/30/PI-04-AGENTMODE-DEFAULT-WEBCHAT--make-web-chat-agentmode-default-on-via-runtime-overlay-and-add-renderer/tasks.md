# Tasks

## Completed

- [x] Remove the dead `--enable-agentmode` flag from `cmd/web-chat/main.go`
- [x] Confirm no tests or docs still rely on `--enable-agentmode`
- [x] Add a default web-chat runtime-policy overlay that injects `agentmode` for every resolved runtime
- [x] Preserve per-profile opt-out by honoring `enabled: false` on the `agentmode` middleware entry
- [x] Define the merge semantics between default runtime middleware and profile runtime middleware, including config override rules
- [x] Ensure `sanitize_yaml` and `default_mode` can still be overridden per profile when the default overlay is present
- [x] Apply the same effective runtime policy to both middleware composition and the agentmode structuredsink wrapper
- [x] Add tests for chat-request runtime resolution when no profile runtime exists
- [x] Add tests for chat-request runtime resolution when a profile explicitly disables agentmode
- [x] Add tests for config override behavior on top of the default overlay
- [x] Update profile documentation to show default-on behavior and `enabled: false` opt-out
- [x] Identify which profile registry sources are used by default when `--profile-registries` is absent and document that behavior clearly
- [x] Add a dedicated `agent_mode` renderer to the web-chat frontend
- [x] Register that renderer in the chat widget renderer registry
- [x] Add a frontend story or fixture demonstrating agent mode switch rendering
- [x] Verify that existing `agent.mode` SEM mapping remains sufficient and no backend translator change is needed
- [x] Run focused backend tests for `cmd/web-chat`, runtime resolution, and agentmode wrapper behavior
- [x] Run frontend tests or build verification for the new `agent_mode` renderer
- [x] Update the ticket docs, changelog, and implementation notes as code lands

## Notes

- Runtime-policy merge now happens in `cmd/web-chat/profile_policy.go` by starting from a default runtime that contains `agentmode`, then replaying the resolved profile stack from `profile.stack.lineage`.
- Middleware merge keys are normalized `(name, id)` pairs. Later stack layers replace earlier entries with the same key.
- Tool names are merged as an ordered unique union from base to leaf.
- System prompt resolution remains “last non-empty wins”.
