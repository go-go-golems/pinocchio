---
Title: Pinocchio Webchat Symbol Migration Playbook
Slug: webchat-symbol-migration-playbook
Short: Upgrade guide for the webchat resolver/composer/DTO renames after compatibility symbols were removed.
Topics:
- pinocchio
- webchat
- migration
- api
- thirdparty
Commands:
- web-chat
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# Webchat Symbol Migration Playbook

## Scope

This guide covers the GP-02 webchat rename set in `cmd/web-chat`, `pkg/webchat`, and `pkg/webchat/http`.

No compatibility aliases are kept for these symbols. Consumers must migrate to the canonical names below.

## Type and Function Renames

| Old | New |
|---|---|
| `webChatProfileResolver` | `ProfileRequestResolver` |
| `newWebChatProfileResolver` | `newProfileRequestResolver` |
| `webChatRuntimeComposer` | `ProfileRuntimeComposer` |
| `newWebChatRuntimeComposer` | `newProfileRuntimeComposer` |
| `ConversationRequestPlan` | `ResolvedConversationRequest` |
| `AppConversationRequest` | `ConversationRuntimeRequest` |
| `resolveProfile` | `resolveProfileSelection` |
| `runtimeKeyFromPath` | `profileSlugFromPath` |
| `baseOverridesForProfile` | `runtimeDefaultsFromProfile` |
| `mergeOverrides` | `mergeRuntimeOverrides` |
| `newInMemoryProfileRegistry` | `newInMemoryProfileService` |
| `registerProfileHandlers` | `registerProfileAPIHandlers` |
| `runtimeFingerprintPayload` | `RuntimeFingerprintInput` |
| `runtimeFingerprint` | `buildRuntimeFingerprint` |
| `validateOverrides` | `validateRuntimeOverrides` |
| `parseMiddlewareOverrides` | `parseRuntimeMiddlewareOverrides` |
| `parseToolOverrides` | `parseRuntimeToolOverrides` |
| `defaultWebChatRegistrySlug` | `defaultRegistrySlug` |

## Request Contract Field Renames

`pinocchio/pkg/inference/runtime/ConversationRuntimeRequest` fields were renamed:

| Old field | New field |
|---|---|
| `RuntimeKey` | `ProfileKey` |
| `ResolvedRuntime` | `ResolvedProfileRuntime` |
| `Overrides` | `RuntimeOverrides` |

## Cookie Literal Cleanup

The repeated `chat_profile` literal is now centralized:

- `currentProfileCookieName` in `cmd/web-chat/profile_policy.go`

## Migration Steps

1. Replace symbol names using the mapping above.
2. Update any `infruntime.ConversationRuntimeRequest` literal fields:
   - `RuntimeKey:` -> `ProfileKey:`
   - `ResolvedRuntime:` -> `ResolvedProfileRuntime:`
   - `Overrides:` -> `RuntimeOverrides:`
3. Re-run tests and fix any remaining compile errors.

## Search Commands

```bash
rg -n "webChatProfileResolver|newWebChatProfileResolver|webChatRuntimeComposer|newWebChatRuntimeComposer|ConversationRequestPlan|AppConversationRequest|runtimeKeyFromPath|baseOverridesForProfile|mergeOverrides|newInMemoryProfileRegistry|registerProfileHandlers|runtimeFingerprintPayload|runtimeFingerprint\\(|validateOverrides\\(|parseMiddlewareOverrides\\(|parseToolOverrides\\("
```

```bash
rg -n "ConversationRuntimeRequest\\{|\\.RuntimeKey|\\.ResolvedRuntime|\\.Overrides"
```

## Validation

```bash
go test ./cmd/web-chat -count=1
go test ./pkg/webchat/... -count=1
go test ./pkg/inference/runtime -count=1
go test ./... -count=1
```

## Troubleshooting

| Problem | Cause | Fix |
|---|---|---|
| `undefined: webhttp.ConversationRequestPlan` | Type rename | Use `webhttp.ResolvedConversationRequest` |
| `undefined: webchat.AppConversationRequest` | Type rename | Use `webchat.ConversationRuntimeRequest` |
| unknown field `RuntimeKey` in `ConversationRuntimeRequest` | Field rename in runtime contract | Use `ProfileKey` |
| unknown field `ResolvedRuntime` in `ConversationRuntimeRequest` | Field rename in runtime contract | Use `ResolvedProfileRuntime` |
| unknown field `Overrides` in `ConversationRuntimeRequest` | Field rename in runtime contract | Use `RuntimeOverrides` |

## See Also

- `pinocchio/pkg/doc/topics/runtime-symbol-migration-playbook.md`
- `pinocchio/pkg/doc/topics/webchat-backend-reference.md`
- `pinocchio/pkg/doc/topics/webchat-framework-guide.md`
