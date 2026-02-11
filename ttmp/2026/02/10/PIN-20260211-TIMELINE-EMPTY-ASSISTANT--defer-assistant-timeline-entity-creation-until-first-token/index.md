---
Title: Defer assistant timeline entity creation until first token
Ticket: PIN-20260211-TIMELINE-EMPTY-ASSISTANT
Status: complete
Topics:
    - pinocchio
    - bug
    - chat
    - backend
    - analysis
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go
    - Path: pkg/ui/backend.go
      Note: Primary location for deferred assistant creation follow-up
    - Path: pkg/webchat/sem_translator.go
      Note: Semantic event order context for llm.start/llm.delta
    - Path: pkg/webchat/timeline_projector.go
      Note: Secondary path to keep behavior consistent
ExternalSources: []
Summary: Investigation ticket documenting why thinking-model sessions show an empty assistant timeline block before first assistant token and recommending deferred assistant entity creation.
LastUpdated: 2026-02-10T20:26:16.586351574-05:00
WhatFor: ""
WhenToUse: ""
---



# Defer assistant timeline entity creation until first token

## Overview

This ticket investigates a timeline UX bug in `cmd/pinocchio` where assistant entities appear empty before thinking output. The analysis identifies eager assistant entity creation on stream-start events as the primary cause and provides a recommended deferred-creation strategy.

## Status

- Type: analysis/research
- Code changes: not part of this ticket yet
- Output: analysis + diary + reMarkable upload
