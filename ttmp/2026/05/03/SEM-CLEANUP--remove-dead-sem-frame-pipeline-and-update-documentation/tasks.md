---
Title: Tasks
Ticket: SEM-CLEANUP
DocType: tasks
---

## Tasks

### Dead code removal
- [x] 1. Delete `pkg/sem/registry/` (dead Go package, zero consumers)
- [x] 2. Migrate `ChatWidget.stories.tsx` away from SEM registry imports
- [x] 3. Delete `sem/registry.ts` and `sem/registry.test.ts` (dead TS modules)
- [x] 4. Move `sem/timelinePropsRegistry.ts` to `webchat/timelinePropsRegistry.ts` and update imports

### Obsolete documentation
- [x] 5. Delete `pkg/doc/tutorials/04-intern-app-owned-middleware-events-timeline-widgets.md`
- [x] 6. Remove cross-reference to tutorial 04 from tutorial 09
- [x] 7. Update `pkg/doc/topics/webchat-frontend-integration.md` (replace SEM sections)
- [x] 8. Update `pkg/doc/topics/webchat-frontend-architecture.md` (replace SEM pipeline)
- [x] 9. Update `pkg/doc/topics/13-js-api-reference.md` (replace SEM API references)
- [x] 10. Update `pkg/doc/topics/webchat-debugging-and-ops.md` (remove onSem reference)

### Debug UI migration to sessionstream
- [x] 11. Replace `debug-ui/ws/debugTimelineWsManager.ts` with sessionstream WS client (connect to `/api/chat/ws`, send subscribe frame, handle snapshot + ui-event frames)
- [x] 12. Delete `debug-ui/api/debugApi.ts` and `debug-ui/api/debugApi.test.ts` (RTK Query against non-existent endpoints)
- [x] 13. Delete `debug-ui/api/turnParsing.ts` and `debug-ui/api/turnParsing.test.ts` (turn block parser no longer needed)
- [x] 14. Delete `debug-ui/mocks/` (entire directory — MSW mocks for old debug API)
- [x] 15. Rewrite `debug-ui/routes/useLaneData.ts` to read from Redux slice instead of dead API endpoints
- [x] 16. Simplify `debug-ui/components/TimelineLanes.tsx` to 2 lanes (remove StateTrackLane)
- [x] 17. Delete `debug-ui/components/StateTrackLane.tsx` and `debug-ui/components/TurnInspector.tsx`
- [x] 18. Delete or stub `debug-ui/routes/TurnDetailPage.tsx`
- [x] 19. Replace conversation list with session ID text input
- [x] 20. Delete `sem/timelineMapper.ts` (no remaining consumers after debug-ui migration)

### Verification
- [x] 21. `make build && go test ./... -count=1 && cd cmd/web-chat/web && npm run check`
- [x] 22. `grep -rn 'semregistry\|RegisterByType\|sem/registry\|handleSem\|registerSem' pkg/doc/ cmd/web-chat/` returns nothing
- [x] 23. Verify debug-ui works: open `?debug=1`, enter session ID, see snapshot entities and live ui-events
