# Changelog

## 2025-11-18

- Initial workspace created


## 2025-11-18

Set up .ttmp.yaml/vocabulary and documented lint+profile findings in analysis/01-config-and-profile-migration-analysis.md

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/2025-11-18/PIN-20251118/analysis/01-config-and-profile-migration-analysis.md — Primary analysis doc


## 2025-11-18

Completed plan points 1 and 2: Modernized CLI bootstrap and adopted env/config middlewares. Replaced clay.InitViper with clay.InitGlazed, logging.InitLoggerFromViper with logging.InitLoggerFromCobra, and GatherFlagsFromViper with UpdateFromEnv + LoadParametersFromFiles in geppetto layers middleware. Also replaced viper.GetStringSlice for repositories with direct config file reading.

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/../geppetto/pkg/layers/layers.go — Replaced GatherFlagsFromViper with UpdateFromEnv and LoadParametersFromResolvedFilesForCobra
- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/pinocchio/main.go — Replaced Viper initialization with InitGlazed and InitLoggerFromCobra


## 2025-11-18

Converted config file from flat to layered format and added mapper to exclude repositories key. Config file now uses layer-slug structure (ai-chat, openai-chat, etc.) and repositories is handled separately during initialization.

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/../geppetto/pkg/layers/layers.go — Added config mapper to filter out repositories key from layer parsing


## 2025-11-18

Created detailed implementation diary documenting the migration work, including what was tried, what failed, what worked, lessons learned, and what would be done differently


## 2025-11-18

Fixed configFilesResolver to use parsedCommandLayers instead of parsedLayers from middleware, since flags haven't been parsed yet when resolver runs. Also fixed file loading order to load default config first (low precedence), then explicit config (high precedence).

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/../geppetto/pkg/layers/layers.go — Fixed resolver to read --config-file flag from parsedCommandLayers and corrected file loading order


## 2025-11-18

Documented multiple strategies (incl. mini middleware chain) for resolving profile-settings before wiring the profile middleware.

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/2025-11-18/PIN-20251118/analysis/02-profile-preparse-options.md — Profile pre-parse analysis


## 2025-11-18

Updated implementation diary with Phase 5 documenting the configFilesResolver fix - discovered middleware execution order issue and fixed by using parsedCommandLayers instead of parsedLayers


## 2025-11-18

Created comprehensive playbook for migrating applications from Viper to Glazed config system, including step-by-step instructions, code examples, common issues, and verification checklist

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/2025-11-18/PIN-20251118/playbooks/01-migrating-from-viper-to-glazed-config.md — Step-by-step migration playbook based on Pinocchio migration experience


## 2025-11-18

Added whitelist to UpdateFromEnv middleware to restrict environment variable parsing to the same layers that were previously whitelisted for Viper parsing (ai-chat, ai-client, openai-chat, claude-chat, gemini-chat, embeddings, profile-settings)

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/../geppetto/pkg/layers/layers.go — Wrapped UpdateFromEnv with WrapWithWhitelistedLayers to shield other layers from env parsing


## 2025-11-18

Added whitelist to UpdateFromEnv middleware to restrict environment variable parsing to the same layers that were previously whitelisted for Viper parsing (ai-chat, ai-client, openai-chat, claude-chat, gemini-chat, embeddings, profile-settings). This shields other layers from environment variable parsing, matching the previous Viper behavior.

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/../geppetto/pkg/layers/layers.go — Wrapped UpdateFromEnv with WrapWithWhitelistedLayers to restrict env parsing to specific layers


## 2025-11-18

Captured profile parsing plan (mini middleware chain, helper, wiring + tests) in design/01-profile-loading-plan.md and added implementation tasks.

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/2025-11-18/PIN-20251118/design/01-profile-loading-plan.md — Plan doc

