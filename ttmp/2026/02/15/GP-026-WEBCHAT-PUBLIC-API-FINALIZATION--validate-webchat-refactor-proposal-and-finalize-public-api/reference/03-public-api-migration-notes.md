---
Title: Public API Migration Notes
Ticket: GP-026-WEBCHAT-PUBLIC-API-FINALIZATION
Status: active
Topics:
    - webchat
    - migration
    - api
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/webchat/http_helpers.go
      Note: Canonical app-owned HTTP helper constructors
    - Path: pkg/webchat/chat_service.go
      Note: Chat-focused API surface
    - Path: pkg/webchat/stream_hub.go
      Note: Stream lifecycle and websocket attach surface
    - Path: pkg/webchat/timeline_service.go
      Note: Timeline hydration service
    - Path: pkg/webchat/chat/api.go
      Note: New subpackage export surface
    - Path: pkg/webchat/stream/api.go
      Note: New subpackage export surface
    - Path: pkg/webchat/timeline/api.go
      Note: New subpackage export surface
    - Path: pkg/webchat/http/api.go
      Note: New subpackage export surface
    - Path: pkg/webchat/bootstrap/api.go
      Note: New subpackage export surface
ExternalSources: []
Summary: Migration map from legacy webchat router/conversation entry points to split service APIs and subpackage exports.
LastUpdated: 2026-02-15T15:08:00-05:00
WhatFor: Provide releasable migration guidance for consumers moving to the GP-026 public API.
WhenToUse: Use when upgrading app wiring from legacy webchat helpers to split services.
---

# Public API Migration Notes

## Scope

These notes cover the GP-026 migration from legacy app wiring (`ConversationService` + legacy chat/ws helpers) to split services (`ChatService`, `StreamHub`, `TimelineService`) and new subpackage exports.

## New Canonical Entry Points

- Chat HTTP: `webchat.NewChatHTTPHandler(chatSvc, resolver)`
- WS HTTP: `webchat.NewWSHTTPHandler(streamHub, resolver, upgrader)`
- Timeline HTTP: `webchat.NewTimelineHTTPHandler(timelineSvc, logger)`
- Chat service: `webchat.ChatService`
- Stream service: `webchat.StreamHub`
- Timeline service: `webchat.TimelineService`

## Subpackage Exports

- `pkg/webchat/chat`
- `pkg/webchat/stream`
- `pkg/webchat/timeline`
- `pkg/webchat/http` (package name: `webhttp`)
- `pkg/webchat/bootstrap`

These re-export stable service-level APIs while the root package continues to host implementation internals.

## Migration Map

- Legacy `NewChatHandler(...)` -> `NewChatHTTPHandler(...)`
- Legacy `NewWSHandler(...)` -> `NewWSHTTPHandler(...)`
- Legacy app use of `Router.ConversationService()` -> `Router.ChatService()` + `Router.StreamHub()`
- Timeline hydration should be mounted via `TimelineService` + `NewTimelineHTTPHandler` (canonical route remains `/api/timeline`).

## cmd/web-chat and web-agent-example Status

- `cmd/web-chat` now uses `webchat.NewServer(...)`, `srv.ChatService()`, `srv.StreamHub()`, and explicit timeline handler mounting.
- `web-agent-example` now uses the same split-service pattern and includes sink-wrapper behavior tests.

## Compatibility Notes

- `ConversationService` remains present as a compatibility facade for now.
- Legacy helper entry points were removed from package exports in this phase.
- Route ownership remains app-level; package helpers are optional glue.

## Contract Test Coverage

- `pkg/webchat/http_helpers_contract_test.go` verifies chat/ws/timeline helper contracts for:
  - request resolution failures
  - status/response shape behavior
  - timeline snapshot success and error handling

## Recommended Consumer Update Order

1. Switch chat/ws handlers to `NewChatHTTPHandler` and `NewWSHTTPHandler`.
2. Switch wiring to `ChatService` and `StreamHub` accessors.
3. Mount `/api/timeline` via `TimelineService` + `NewTimelineHTTPHandler`.
4. Move imports to `webchat/{chat,stream,timeline,http,bootstrap}` as preferred public namespaces.
