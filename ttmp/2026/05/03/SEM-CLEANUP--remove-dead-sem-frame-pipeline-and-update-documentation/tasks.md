---
Title: Tasks
Ticket: SEM-CLEANUP
DocType: tasks
---

## Tasks

### Dead code removal
- [ ] 1. Delete `pkg/sem/registry/` (dead Go package, zero consumers)
- [ ] 2. Migrate `ChatWidget.stories.tsx` away from SEM registry imports
- [ ] 3. Delete `sem/registry.ts` and `sem/registry.test.ts` (dead TS modules)
- [ ] 4. Move `sem/timelinePropsRegistry.ts` to `webchat/timelinePropsRegistry.ts` and update imports

### Obsolete documentation
- [ ] 5. Delete `pkg/doc/tutorials/04-intern-app-owned-middleware-events-timeline-widgets.md`
- [ ] 6. Remove cross-reference to tutorial 04 from tutorial 09
- [ ] 7. Update `pkg/doc/topics/webchat-frontend-integration.md` (replace SEM sections)
- [ ] 8. Update `pkg/doc/topics/webchat-frontend-architecture.md` (replace SEM pipeline)
- [ ] 9. Update `pkg/doc/topics/13-js-api-reference.md` (replace SEM API references)
- [ ] 10. Update `pkg/doc/topics/webchat-debugging-and-ops.md` (remove onSem reference)

### Debug UI migration to sessionstream
- [ ] 11. Replace `debug-ui/ws/debugTimelineWsManager.ts` with sessionstream WS client (connect to `/api/chat/ws`, send subscribe frame, handle snapshot + ui-event frames)
- [ ] 12. Delete `debug-ui/api/debugApi.ts` and `debug-ui/api/debugApi.test.ts` (RTK Query against non-existent endpoints)
- [ ] 13. Delete `debug-ui/api/turnParsing.ts` and `debug-ui/api/turnParsing.test.ts` (turn block parser no longer needed)
- [ ] 14. Delete `debug-ui/mocks/` (entire directory — MSW mocks for old debug API)
- [ ] 15. Rewrite `debug-ui/routes/useLaneData.ts` to read from Redux slice instead of dead API endpoints
- [ ] 16. Simplify `debug-ui/components/TimelineLanes.tsx` to 2 lanes (remove StateTrackLane)
- [ ] 17. Delete `debug-ui/components/StateTrackLane.tsx` and `debug-ui/components/TurnInspector.tsx`
- [ ] 18. Delete or stub `debug-ui/routes/TurnDetailPage.tsx`
- [ ] 19. Replace conversation list with session ID text input
- [ ] 20. Delete `sem/timelineMapper.ts` (no remaining consumers after debug-ui migration)

### Verification
- [ ] 21. `make build && go test ./... -count=1 && cd cmd/web-chat/web && npm run check`
- [ ] 22. `grep -rn 'semregistry\|RegisterByType\|sem/registry\|handleSem\|registerSem' pkg/doc/ cmd/web-chat/` returns nothing
- [ ] 23. Verify debug-ui works: open `?debug=1`, enter session ID, see snapshot entities and live ui-events
