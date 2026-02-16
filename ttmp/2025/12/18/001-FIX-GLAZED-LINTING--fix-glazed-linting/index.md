---
Title: Fix Glazed Linting
Ticket: 001-FIX-GLAZED-LINTING
Status: complete
Topics:
    - pinocchio
    - glaze
    - config
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/cmd/pinocchio/cmds/catter/catter.go
      Note: Catter command - migrated GatherSpecificFlagsFromViper to UpdateFromEnv
    - Path: pinocchio/cmd/pinocchio/cmds/prompto/prompto.go
      Note: Prompto command - migrated InitViperInstanceWithAppName to direct config reading
    - Path: pinocchio/cmd/pinocchio/main.go
      Note: Main entry point - migrated InitViper to InitGlazed and config reading
    - Path: pinocchio/pkg/cmds/helpers/parse-helpers.go
      Note: Helper function - migrated GatherFlagsFromViper to config middlewares
ExternalSources: []
Summary: ""
LastUpdated: 2026-02-14T20:11:53.366069723-05:00
WhatFor: ""
WhenToUse: ""
---



# Fix Glazed Linting

Document workspace for 001-FIX-GLAZED-LINTING.
