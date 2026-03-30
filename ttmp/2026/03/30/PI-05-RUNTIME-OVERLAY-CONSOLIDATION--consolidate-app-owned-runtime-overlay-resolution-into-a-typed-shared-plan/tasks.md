# Tasks

## Completed

- [x] Audited the current runtime-overlay implementations in `pinocchio/cmd/web-chat`, `go-go-os-chat/pkg/profilechat`, and `gec-rag/internal/webchat`, and wrote down the exact merge differences in the diary
- [x] Chose `pinocchio/pkg/inference/runtime` as the shared helper package
- [x] Defined the shared concept names in code:
  `ResolvedRuntimePlan`, `ResolveRuntimePlanOptions`, `MergeProfileRuntimeOptions`, `ToolMergeMode`, and `RuntimeFingerprintInput`
- [x] Added a typed stack-lineage field to resolved profiles in `geppetto/pkg/engineprofiles`
- [x] Kept the old `profile.stack.lineage` metadata payload for backward compatibility while stopping new runtime code from reparsing it
- [x] Implemented shared runtime-plan resolution in `pinocchio/pkg/inference/runtime/runtime_plan.go`
- [x] Implemented shared deterministic runtime fingerprint generation from typed inputs in `pinocchio/pkg/inference/runtime/runtime_plan.go`
- [x] Defined explicit merge semantics in shared code:
  prompt last-non-empty wins, middleware identity keyed by `name` plus optional `id`, default tool union, optional tool replacement
- [x] Refactored `pinocchio/cmd/web-chat/profile_policy.go` to use the shared typed helper instead of its local stack-replay and merge code
- [x] Refactored `pinocchio/cmd/web-chat/runtime_composer.go` to use the shared fingerprint helper instead of a local marshaled hash payload
- [x] Ported the prerequisite `geppetto` and `pinocchio` framework commits into the `wesen-os` workspace clones so `go-go-os-chat` could consume the new shared API without local replace hacks
- [x] Refactored `go-go-os-chat/pkg/profilechat/request_resolver.go` to use the shared typed helper, upgrading it from leaf-only runtime resolution
- [x] Refactored `go-go-os-chat/pkg/profilechat/runtime_composer.go` to use the shared fingerprint helper
- [x] Implemented a typed inference-runtime-plan seam in `gec-rag/internal/webchat/resolver.go` so the app now matches the shared resolver shape even though it still pins released `pinocchio`
- [x] Replaced the remaining ad hoc runtime fingerprint `map[string]any` in `gec-rag` with a typed struct payload
- [x] Added and ran focused tests in the touched framework and app packages

## Remaining

- [ ] Add a first-class overlay-source abstraction if more apps need pluggable sources beyond base runtime, profile-extension runtime, and local application-profile overlays
- [ ] Publish or bump `pinocchio` and `geppetto` versions so `gec-rag` can consume `ResolveRuntimePlan` directly instead of using the local typed adapter seam
- [ ] Write a focused Pinocchio doc page describing the consolidated runtime-overlay contract, the merge rules, and the remaining justified dynamic map surfaces
