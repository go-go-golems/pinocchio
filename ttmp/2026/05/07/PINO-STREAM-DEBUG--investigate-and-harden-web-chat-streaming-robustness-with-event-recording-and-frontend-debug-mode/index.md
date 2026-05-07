---
Title: "Investigate and Harden Web-Chat Streaming Robustness with Event Recording and Frontend Debug Mode"
Ticket: PINO-STREAM-DEBUG
Status: active
Topics:
    - streaming-robustness
    - event-recording
    - frontend-debug
    - hydration
    - websocket
    - sessionstream
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/web/src/ws/wsManager.ts
      Note: Frontend WebSocket manager — receives, parses, buffers, and dispatches frames
    - Path: cmd/web-chat/web/src/ws/protocol.ts
      Note: Protobuf JSON frame parser and normalizer
    - Path: cmd/web-chat/web/src/store/timelineSlice.ts
      Note: Redux slice for timeline entity upsert/delete/rekey operations
    - Path: cmd/web-chat/web/src/webchat/components/Timeline.tsx
      Note: React timeline renderer — maps entities to renderers
    - Path: cmd/web-chat/web/src/webchat/cards.tsx
      Note: Card renderers for chat messages, thinking blocks, agent mode
    - Path: cmd/web-chat/web/src/webchat/hooks/useStickyScrollFollow.ts
      Note: Sticky bottom-scroll hook for streaming UX
    - Path: pkg/chatapp/chat.go
      Note: Chatapp engine — publishes backend events, runs inference loop
    - Path: pkg/chatapp/features.go
      Note: ChatPlugin interface — ProjectUI, ProjectTimeline, HandleRuntimeEvent
    - Path: pkg/chatapp/plugins/reasoning.go
      Note: Reasoning plugin — backend event to UI/timeline projection
    - Path: pkg/chatapp/plugins/toolcall.go
      Note: Tool call plugin — backend event to UI/timeline projection
    - Path: cmd/web-chat/agentmode_chat_feature.go
      Note: AgentMode plugin — preview/committed mode switches
ExternalSources: []
Summary: "Build investigation and debugging tools for the Pinocchio web-chat streaming pipeline: record all backend events, record all frontend WebSocket frames and hydration snapshots, and provide a debug UI for comparing backend-emitted vs frontend-received event sequences to find discrepancies in parsing, projection, and rendering."
LastUpdated: 2026-05-07T00:30:00-04:00
WhatFor: "When streaming chat responses go wrong — missing entities, wrong order, lost events on reload, hydration mismatches — there is no structured way to compare what the backend emitted versus what the frontend received and rendered. This ticket creates that investigation layer."
WhenToUse: "Use when debugging streaming discrepancies, hydration bugs, event ordering issues, or rendering gaps in the Pinocchio web-chat."
---



# Investigate and Harden Web-Chat Streaming Robustness with Event Recording and Frontend Debug Mode

## Overview

This ticket creates a structured investigation and debugging layer for the Pinocchio web-chat streaming pipeline. The goal is to make it possible to compare, at every stage, what the backend emitted, what the WebSocket transported, what the frontend parsed, what the projections produced, and what the Redux store rendered.

## Status

Current status: **active**

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Investigation diary, API contracts, context summaries
- playbooks/ - Debug procedures and test scenarios
- scripts/ - Investigation scripts and tools
