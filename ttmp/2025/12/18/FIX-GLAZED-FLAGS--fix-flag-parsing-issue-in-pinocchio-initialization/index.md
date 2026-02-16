---
Title: Fix flag parsing issue in pinocchio initialization
Ticket: FIX-GLAZED-FLAGS
Status: complete
Topics:
    - bug
    - flags
    - initialization
DocType: index
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: cmd/pinocchio/main.go
      Note: |-
        Main fix: early logging flag pre-parse + debug dump; avoids help/unknown-flag failures
        Now uses glazed InitEarlyLoggingFromArgs helper
ExternalSources: []
Summary: ""
LastUpdated: 2026-02-14T20:12:02.344983932-05:00
WhatFor: ""
WhenToUse: ""
---





# Fix flag parsing issue in pinocchio initialization

Document workspace for FIX-GLAZED-FLAGS.
