# Tasks

## TODO

- [x] Create broad debug persistence inventory document covering conversation, timeline, turns, raw events, transport, runtime, and error domains
- [x] Create restricted implementation plan document focused on persisting conversation index in existing timeline store
- [x] Create implementation diary document and keep it updated per implementation step
- [x] Add `ConversationRecord` model and extend `TimelineStore` interface with conversation index APIs (`UpsertConversation`, `GetConversation`, `ListConversations`)
- [x] Add SQLite migration for `timeline_conversations` table and indexes
- [x] Add in-memory timeline store parity for conversation index
- [x] Wire conversation index writes from `ConvManager` lifecycle touch points
- [x] Update `/api/debug/conversations` and `/api/debug/conversations/:id` to merge live + persisted conversation records
- [ ] Add test coverage for persisted-only and merged live/persisted debug conversation responses
- [ ] Decide whether turn enrichment should be phase-1 (`TurnStore` helper) or deferred
- [x] Upload restricted implementation plan to reMarkable under ticket folder and record artifact path
