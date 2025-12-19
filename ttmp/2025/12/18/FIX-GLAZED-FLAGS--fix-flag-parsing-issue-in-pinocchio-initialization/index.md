---
Title: Fix flag parsing issue in pinocchio initialization
Ticket: FIX-GLAZED-FLAGS
Status: active
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
LastUpdated: 2025-12-18T19:02:28.300746541-05:00
---



# Fix flag parsing issue in pinocchio initialization

Document workspace for FIX-GLAZED-FLAGS.
