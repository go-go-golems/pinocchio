---
Title: Add OCR Verb - Image Attachment Analysis
Ticket: 001-ADD-OCR-VERB
Status: active
Topics:
    - backend
    - cli
    - llm-providers
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/ttmp/2025/12/24/001-ADD-OCR-VERB--add-ocr-verb-image-attachment-analysis/analysis/01-image-attachment-flow-analysis.md
      Note: Comprehensive flow analysis and fix requirements
    - Path: geppetto/ttmp/2025/12/24/001-ADD-OCR-VERB--add-ocr-verb-image-attachment-analysis/reference/01-diary.md
      Note: Step-by-step analysis diary
ExternalSources: []
Summary: ""
LastUpdated: 2025-12-24T14:16:19.481528561-05:00
WhatFor: ""
WhenToUse: ""
---


# Add OCR Verb - Image Attachment Analysis

## Overview

**Goal**: Analyze how image attachments flow from CLI (`--images` flag) through pinocchio/geppetto to LLM providers, validate the end-to-end flow, and identify where images are being lost.

**Status**: Analysis complete. Bug identified and documented.

**Key Finding**: Images are parsed correctly from CLI but never passed to the turn builder. The `buildInitialTurnFromBlocks` function uses `NewUserTextBlock` (no images) instead of `NewUserMultimodalBlock`. Image paths are extracted in `cmd.go` but never used.

**Fix Required**: 
1. Pass image paths through call chain: `RunIntoWriter` → `buildInitialTurn` → `buildInitialTurnFromBlocksRendered` → `buildInitialTurnFromBlocks`
2. Create helper function to convert image paths → turn payload format (`[]map[string]any{"media_type": string, "content": []byte}`)
3. Use `NewUserMultimodalBlock` when images are present, `NewUserTextBlock` otherwise (backward compatible)

**Documents**:
- [Diary](./reference/01-diary.md) - Step-by-step analysis with code references
- [Flow Analysis](./analysis/01-image-attachment-flow-analysis.md) - Comprehensive analysis with fix requirements

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- backend
- cli
- llm-providers

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
