---
Title: Investigation Diary
Ticket: SEM-CLEANUP
DocType: reference
Status: active
---

# Investigation Diary

## 2026-05-03: Initial Investigation

### What was investigated

The user asked whether SEM frames are still used in the codebase after the webchat was cleaned up to use sessionstream. Specifically, they pointed to tutorial 04 as potentially obsolete.

### What was found

1. **Go SEM registry (`pkg/sem/registry/registry.go`)**: Zero consumers. `RegisterByType` and `Handle` are never called anywhere. No file imports this package.

2. **TypeScript SEM registry (`sem/registry.ts`)**: Only imported by `ChatWidget.stories.tsx`. The production app (`wsManager.ts`) does not use it at all.

3. **Tutorial 04**: Entirely about the old SEM pipeline (1041 lines). References `semregistry.RegisterByType`, `wrapSem`, `registerSem`, and the entire SEM frame lifecycle. Completely superseded by tutorial 09.

4. **Four doc topics** have stale SEM references: `webchat-frontend-integration.md`, `webchat-frontend-architecture.md`, `13-js-api-reference.md`, `webchat-debugging-and-ops.md`.

5. **`timelineMapper.ts` and `timelinePropsRegistry.ts`**: Still active, used by the debug UI and the public webchat export surface respectively. These are not SEM-specific — just misplaced under the `sem/` directory.

6. **`pkg/sem/pb/`**: Protobuf-generated types still widely imported. Must not be touched.

### What worked

- `grep -rn` for import paths and function names gave clear evidence of dead code.
- Checking the Storybook story as the only remaining consumer of the TS registry was straightforward.

### What was tricky

- The debug UI (`debugTimelineWsManager.ts`) still processes `{ sem: true }` envelopes on the WebSocket, which means the debug endpoint might still emit SEM-style frames from the backend. This needs further investigation during implementation.

### Commands used for evidence gathering

```bash
# Check for Go SEM registry consumers
grep -rn 'RegisterByType|semregistry' --include="*.go" .
grep -rn '"github.com/go-go-golems/pinocchio/pkg/sem' --include="*.go" .

# Check for TS SEM registry consumers
grep -rn 'from.*sem/registry' --include="*.ts" --include="*.tsx" cmd/web-chat/web/src/

# Check what the production frontend uses
grep -n 'sem\|SEM\|registerSem\|handleSem' cmd/web-chat/web/src/ws/wsManager.ts

# Check what uses timelineMapper and timelinePropsRegistry
grep -rn 'timelineMapper\|timelinePropsRegistry' --include="*.ts" --include="*.tsx" cmd/web-chat/web/src/

# Check doc references
grep -rl 'SEM\|sem frame' pkg/doc/
```

### Next steps

1. Create ticket with full analysis document.
2. Upload to reMarkable.
3. Execute phased cleanup.

---

## 2026-05-03: Execution Phase

Starting execution of the 8-phase cleanup plan. Working task by task, committing at appropriate intervals.

### Commit plan

1. Delete Go SEM registry (`pkg/sem/registry/`)
2. TS cleanup: migrate Storybook + delete SEM registry + move timelinePropsRegistry
3. Delete obsolete tutorial + update cross-reference
4. Update stale doc topics
5. Debug UI migration to sessionstream
6. Final verification
