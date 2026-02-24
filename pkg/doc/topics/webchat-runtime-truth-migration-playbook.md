---
Title: Webchat Runtime Truth Migration Playbook
Slug: webchat-runtime-truth-migration-playbook
Short: Migrate and validate per-turn runtime truth with conversation current-runtime semantics.
Topics:
- webchat
- migration
- persistence
- sqlite
- debugging
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Purpose

This playbook explains the runtime-semantics cutover:

- turn rows are authoritative (`turns.runtime_key`, `turns.inference_id`),
- conversation rows expose only current pointer semantics (`current_runtime_key` in debug APIs).

Use this when upgrading existing SQLite stores or validating runtime-switch correctness.

## Semantic Model

Two fields now represent different truths:

- per-turn truth: `turns.runtime_key` and `turns.inference_id`.
- conversation pointer: `/api/debug/conversations` and `/api/debug/conversations/:id` return `current_runtime_key`.

Do not treat conversation current runtime as full turn history.

## API Contract Changes

Conversation debug payloads:

- old key: `runtime_key`
- new key: `current_runtime_key`

Turn debug payloads:

- `/api/debug/turns` items include `runtime_key` and `inference_id`.
- `/api/debug/turn/:conv/:session/:turn` phase items include `runtime_key` and `inference_id`.

## Migration and Validation SQL

Inspect schema columns:

```sql
PRAGMA table_info(turns);
```

Expected additive columns:

- `runtime_key TEXT NOT NULL DEFAULT ''`
- `inference_id TEXT NOT NULL DEFAULT ''`

Check indexes:

```sql
SELECT name, sql
FROM sqlite_master
WHERE type = 'index'
  AND tbl_name = 'turns'
  AND name IN ('turns_by_conv_runtime_updated', 'turns_by_conv_inference_updated');
```

Validate runtime history for one conversation:

```sql
SELECT turn_id, runtime_key, inference_id, updated_at_ms
FROM turns
WHERE conv_id = 'your-conv-id'
ORDER BY updated_at_ms ASC, turn_id ASC;
```

Detect rows that still have empty backfill values:

```sql
SELECT COUNT(*) AS missing_runtime_or_inference
FROM turns
WHERE COALESCE(runtime_key, '') = ''
   OR COALESCE(inference_id, '') = '';
```

Inspect candidate metadata for manual recovery:

```sql
SELECT conv_id, session_id, turn_id, turn_metadata_json
FROM turns
WHERE COALESCE(runtime_key, '') = ''
   OR COALESCE(inference_id, '') = ''
LIMIT 50;
```

## Troubleshooting Incomplete Backfill

Backfill is best-effort and uses turn metadata keys. Some old rows may remain empty when metadata was missing or non-canonical.

Recommended response:

1. keep empty string sentinel for unknown historical rows,
2. avoid fabricating runtime history from conversation-level latest pointer,
3. use post-cutover rows for reliable runtime/inference analytics.

## Release Notes Template

Use this snippet in release notes:

```text
Runtime persistence semantics changed:
- per-turn runtime/inference are now first-class and queryable in turns.db,
- conversation debug APIs now expose current_runtime_key (latest pointer only),
- consumers must not infer historical runtime from conversation-level fields.
```

## See Also

- [Webchat Debugging and Operations](webchat-debugging-and-ops.md)
- [Webchat Profile Registry](webchat-profile-registry.md)
