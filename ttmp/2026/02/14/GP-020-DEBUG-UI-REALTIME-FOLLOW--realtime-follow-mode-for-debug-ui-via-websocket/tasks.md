# Tasks

## TODO

- [x] Add follow-mode state/actions/selectors in debug-ui uiSlice
- [x] Implement debug timeline websocket manager with conversation-scoped connect/disconnect
- [x] Implement bootstrap (`/api/timeline` canonical only) then buffered replay ordering for live attach
- [x] Decode `timeline.upsert` and upsert generic timeline entities with dedupe by version/entity
- [x] Add follow controls in SessionList and status badge in app shell
- [x] Support pause/resume/reconnect UX for follow mode
- [x] Ensure read-only behavior (no outbound control messages)
- [x] Ensure follow mode respects app base prefix/root mount for `/ws` and `/api/timeline`
- [ ] Add websocket lifecycle and two-tab follow integration tests (timeline upsert path)
