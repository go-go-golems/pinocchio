---
Title: Webchat Profile Registry Guide
Slug: webchat-profile-registry
Short: Detailed guide to profile registry wiring, selection precedence, and CRUD APIs for pinocchio webchat apps.
Topics:
- webchat
- profiles
- registry
- http
- backend
Commands:
- web-chat
Flags:
- profile
- profile-file
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## What This Page Covers

This page is the reference for profile registry behavior in pinocchio webchat applications. It explains:

- how profile registries are bootstrapped and injected,
- how profile/registry selection is resolved from request inputs,
- how runtime composition uses resolved profile data,
- how profile CRUD endpoints behave and what errors they return.

Use this page when building app-owned chat backends that mount `pkg/webchat/http` handlers.

## Architecture at a Glance

Profile registry support spans three layers:

1. **Geppetto profile domain** (`geppetto/pkg/profiles`)
2. **Request policy resolver** (app-owned, maps HTTP request to profile/runtime selection)
3. **Runtime composer** (app-owned, composes engine from resolved profile runtime + policy output)

In handler-first webchat integration, `/chat` and `/ws` always flow through your resolver before runtime composition.

## Registry Bootstrap

Common bootstrap pattern:

```go
store := gepprofiles.NewInMemoryProfileStore()

registry := &gepprofiles.ProfileRegistry{
  Slug:               gepprofiles.MustRegistrySlug("default"),
  DefaultProfileSlug: gepprofiles.MustProfileSlug("default"),
  Profiles: map[gepprofiles.ProfileSlug]*gepprofiles.Profile{
    gepprofiles.MustProfileSlug("default"): {
      Slug: gepprofiles.MustProfileSlug("default"),
      Runtime: gepprofiles.RuntimeSpec{
        SystemPrompt: "You are an assistant.",
      },
    },
  },
}

_ = store.UpsertRegistry(ctx, registry, gepprofiles.SaveOptions{Actor: "app", Source: "bootstrap"})
profileRegistry, _ := gepprofiles.NewStoreRegistry(store, gepprofiles.MustRegistrySlug("default"))
```

SQLite-backed stores follow the same service interface and are recommended when profiles are edited at runtime.

## Request Selection Precedence

For chat/websocket requests, the typical profile selection order is:

1. explicit body field (`profile`)
2. query field (`profile`)
3. query field (`runtime`) for backward client compatibility
4. `chat_profile` cookie
5. registry default profile

Registry selection commonly follows:

1. explicit body field (`registry`)
2. query field (`registry`)
3. resolver default registry slug

If selection is invalid, resolvers should return `RequestResolutionError` with precise client status (`400`, `404`, or `500`).

## Runtime Composition Contract

Resolvers should populate:

- `RuntimeKey`
- `ProfileVersion`
- `ResolvedRuntime`
- `Overrides` (if allowed by policy)

Runtime composers should consume these fields to produce:

- engine + sink
- runtime key
- runtime fingerprint
- seed system prompt
- allowed tools

To guarantee rebuild on profile updates, include version-sensitive data (`ProfileVersion` and effective runtime inputs) in the fingerprint payload.

## CRUD HTTP APIs

Mount reusable handlers with:

```go
webhttp.RegisterProfileAPIHandlers(mux, profileRegistry, webhttp.ProfileAPIHandlerOptions{
  DefaultRegistrySlug:             gepprofiles.MustRegistrySlug("default"),
  EnableCurrentProfileCookieRoute: true,
  WriteActor:                      "my-app",
  WriteSource:                     "http-api",
  MiddlewareDefinitions:           middlewareDefinitions,
  ExtensionCodecRegistry:          extensionCodecRegistry,
})
```

### Endpoints

| Endpoint | Methods | Purpose |
|---|---|---|
| `/api/chat/profiles` | `GET`, `POST` | list/create profiles |
| `/api/chat/profiles/{slug}` | `GET`, `PATCH`, `DELETE` | read/update/delete a profile |
| `/api/chat/profiles/{slug}/default` | `POST` | set default profile |
| `/api/chat/profile` | `GET`, `POST` | current-profile cookie read/write (optional route) |
| `/api/chat/schemas/middlewares` | `GET` | discover middleware JSON schema contracts |
| `/api/chat/schemas/extensions` | `GET` | discover extension JSON schema contracts |

### Payload Reference

`GET /api/chat/profiles` returns lightweight list items:

```json
[
  {
    "slug": "default",
    "display_name": "Default",
    "description": "General assistant profile",
    "is_default": true,
    "version": 4
  }
]
```

List responses are always JSON arrays sorted by profile slug.

`POST /api/chat/profiles` create payload:

```json
{
  "registry": "default",
  "slug": "analyst",
  "display_name": "Analyst",
  "description": "Data analysis profile",
  "runtime": {
    "system_prompt": "You are an analyst.",
    "tools": ["inventory.list"]
  },
  "policy": {
    "allow_overrides": true,
    "read_only": false
  },
  "set_default": false,
  "expected_version": 0
}
```

`PATCH /api/chat/profiles/{slug}` patch payload:

```json
{
  "display_name": "Analyst v2",
  "runtime": {
    "system_prompt": "You are an analyst. Be concise."
  },
  "expected_version": 1
}
```

`POST /api/chat/profiles/{slug}/default` payload:

```json
{
  "registry": "default",
  "expected_version": 7
}
```

`POST /api/chat/profile` cookie-selection payload:

```json
{
  "slug": "analyst"
}
```

## Current Profile vs Conversation Runtime

`/api/chat/profile` stores UI selection state (cookie), but runtime truth is per turn:

- each chat/websocket request resolves profile at request time,
- runtime key and resolved runtime are attached to the request plan,
- conversation state alone is not sufficient to infer the runtime of every turn.

When profile selection changes mid-conversation, subsequent turns use the new profile while prior turns remain attributable to their original runtime selection.

Debug API interpretation:

- `/api/debug/conversations/:conv_id` -> `current_runtime_key` (latest pointer),
- `/api/debug/turns?conv_id=...` -> per-turn `runtime_key` and `inference_id`.

## Write-Time Validation and Schema APIs

Profile create/update routes enforce middleware correctness before persistence:

- unknown middleware names return `400`,
- middleware `config` payloads are validated/coerced against definition JSON schemas,
- validation errors include field paths such as `runtime.middlewares[0].config`.

Schema endpoints are intended for frontend form-generation and preflight validation:

- `GET /api/chat/schemas/middlewares`
- `GET /api/chat/schemas/extensions`

Middleware schema response example:

```json
[
  {
    "name": "agentmode",
    "version": 1,
    "display_name": "Agent Mode",
    "description": "Injects mode guidance and parses mode switch markers.",
    "schema": {
      "type": "object",
      "properties": {
        "default_mode": { "type": "string" }
      }
    }
  }
]
```

Extension schema response example:

```json
[
  {
    "key": "middleware.config.agentmode@v1",
    "schema": {
      "type": "object",
      "properties": {
        "instances": {
          "type": "object",
          "additionalProperties": { "type": "object" }
        }
      },
      "required": ["instances"],
      "additionalProperties": false
    }
  }
]
```

Extension schema merge precedence:

1. explicit schemas passed in handler options (`ExtensionSchemas`),
2. middleware-derived typed keys from middleware definitions,
3. codec-derived schemas from codec registries that implement `ExtensionCodecLister` and codecs implementing `ExtensionSchemaCodec`.

Use these schemas to build profile-editing UIs that avoid sending invalid payloads.

## Error Semantics

Profile API handlers map typed profile errors to stable HTTP status classes:

- not found -> `404`
- validation failure -> `400`
- policy violation -> `403`
- optimistic concurrency conflict -> `409`
- unhandled/store failures -> `500`

When writing clients, key behavior primarily from status codes and error class, not only exact text.

## Resolver and Policy Guidance

Recommended resolver behavior:

- reject malformed JSON as `400`,
- reject disallowed runtime overrides early,
- keep conv-id generation deterministic (body `conv_id` or generated UUID),
- map profile and registry failures to typed `RequestResolutionError`.

Recommended policy behavior:

- keep override policy in profile definitions (`allow_overrides`, denied keys),
- use read-only profiles for protected runtime presets,
- enforce expected-version for mutable APIs to avoid last-write-wins races.

## Testing Recommendations

Minimum integration coverage for profile-aware webchat:

- list -> select -> chat request uses selected runtime key,
- create profile -> appears in list -> usable immediately,
- patch profile increments version and drives runtime rebuild behavior,
- read-only profile mutation is rejected.

These tests should run against app-mounted handlers, not only isolated service functions.

## Hard-Cutover Notes

Current rollout assumptions:

- profile-registry middleware integration is always enabled,
- `PINOCCHIO_ENABLE_PROFILE_REGISTRY_MIDDLEWARE` is removed,
- compatibility aliases for renamed runtime/webchat symbols are removed,
- profile CRUD + schema endpoints are the canonical integration surface.

Do not gate behavior with legacy env toggles. If a deployment needs rollback, use release rollback and profile DB snapshot restore.

## SQLite Operations and Rollout Notes

For durable multi-user profile editing, run web-chat with SQLite-backed registry storage:

```bash
pinocchio web-chat --profile-registry-db ./data/profiles.db
```

Operational recommendations:

- keep timestamped DB backups before bulk profile edits,
- restrict DB and backup file permissions to service operators,
- validate restore drills with `GET /api/chat/profiles` and one explicit profile-selected chat request.

Migration/rollout posture:

- registry middleware integration is always enabled,
- rollback uses release rollback + profile DB snapshot restore, not runtime env toggles.

## Go-Go-OS Integration Notes

Go-go-os inventory chat reuses the same shared profile API handlers from `pinocchio/pkg/webchat/http`:

- same CRUD endpoints and status/error model,
- same current-profile route behavior,
- same middleware/extension schema endpoint contracts.

This keeps frontend profile editors portable across pinocchio web-chat and go-go-os app backends.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `/chat` ignores selected profile | Resolver returns fixed runtime key | Ensure resolver reads body/query/cookie and resolves profile through registry |
| profile updates do not trigger runtime change | Fingerprint missing profile version/effective runtime inputs | Add `ProfileVersion` and runtime inputs to fingerprint payload |
| `/api/chat/profiles` returns empty list unexpectedly | Bootstrap registry did not upsert expected profiles | Validate bootstrap sequence and registry slug |
| create/patch returns `validation error (runtime.middlewares[*].name)` | middleware not registered in runtime definition registry | inspect application middleware-definition wiring and middleware name |
| create/patch returns `validation error (runtime.middlewares[*].config)` | payload does not satisfy middleware schema | fetch `/api/chat/schemas/middlewares` and fix payload |
| mutation returns conflict | stale `expected_version` | read latest profile and retry with current version |
| mutation unexpectedly forbidden | profile policy is read-only or denies operation | inspect profile policy and update intentionally |

## See Also

- [Webchat Framework Guide](webchat-framework-guide.md)
- [Webchat User Guide](webchat-user-guide.md)
- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)
- [Webchat Symbol Migration Playbook](webchat-symbol-migration-playbook.md)
- `geppetto/pkg/doc/playbooks/06-operate-sqlite-profile-registry.md`
