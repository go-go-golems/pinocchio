# Tasks

## TODO

- [x] Create shared `pkg/persistence/chatstore` package and migrate timeline store interfaces/implementations/tests
- [x] Migrate turn store interfaces/implementations and add dedicated SQLite turn store tests in new package
- [x] Update `pkg/webchat` to consume `pkg/persistence/chatstore` (hard cutoff, no compatibility shims)
- [x] Update `cmd/web-chat` timeline DB command imports/usages to the new package
- [x] Compile and test `pinocchio` packages affected by migration
- [x] Compile and test `web-chat-example` against migrated APIs and fix any fallout
