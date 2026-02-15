# Tasks

## TODO

- [ ] Introduce `ConversationWSBroker` abstraction and transport adapter over `ConnectionPool`
- [ ] Route existing SEM stream fanout through broker publish API
- [ ] Route timeline upsert emission through broker publish API
- [ ] Add connection subscription model (`channels`) parsed at `/ws` connect
- [ ] Add channel classification rules for outgoing frames (`sem`, `timeline`, `control`)
- [ ] Add broker-level filtering by channel
- [ ] Keep websocket `seq` global per conversation (no per-connection renumbering after filtering)
- [ ] Add router option for backend websocket emitter factory registration
- [ ] Add default compatibility channel set matching current behavior when `channels` is omitted
- [ ] Add unit and integration tests for channel-filtered fanout and compatibility
- [ ] Add observability counters/log fields for publish/deliver/drop paths
- [ ] Keep `debug.turn_snapshot` explicitly deferred from first implementation (documented follow-up only)
- [ ] Update developer docs for websocket protocol and extension contracts
