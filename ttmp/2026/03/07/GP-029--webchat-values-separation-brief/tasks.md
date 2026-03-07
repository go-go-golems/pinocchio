# Tasks

## Completed

- [x] Create Pinocchio ticket workspace under `pinocchio/ttmp`
- [x] Write colleague brief for separating Glazed values parsing from `pkg/webchat.Router`
- [x] Prepare the ticket for validation and reMarkable upload

## In Progress

- [ ] Create an implementation diary and turn the brief into an executable backlog
- [ ] Refactor `pkg/webchat` so core router/server construction works from explicit dependencies
- [ ] Preserve parsed-values convenience constructors as thin adapters
- [ ] Update tests to cover both dependency-injected and parsed-values construction
- [ ] Write a migration guide in `pkg/doc` for embedders moving to the new constructor layering
- [ ] Validate with focused `go test` coverage and ticket hygiene checks

## Task Breakdown

### 1. Ticket and diary setup

- [ ] Add a diary reference document for step-by-step implementation notes
- [ ] Update the ticket index to link the diary and implementation status
- [ ] Expand the task list from the brief into concrete implementation tasks

### 2. Stream backend constructor split

- [ ] Add an explicit stream-backend constructor that accepts already-decoded Redis settings
- [ ] Keep `NewStreamBackendFromValues(...)` as a thin adapter around the explicit constructor
- [ ] Update stream backend tests to cover both entry points

### 3. Router constructor split

- [ ] Introduce an explicit dependency-injected router constructor
- [ ] Introduce an adapter helper that builds router dependencies from `*values.Values`
- [ ] Make `NewRouter(...)` delegate to the adapter helper instead of decoding values directly
- [ ] Remove direct `DecodeSectionInto(...)` calls from the core router constructor
- [ ] Ensure `BuildHTTPServer()` no longer depends on retaining parsed values on the router

### 4. Server constructor split

- [ ] Introduce a dependency-injected server constructor path
- [ ] Make `NewServer(...)` delegate through the parsed-values adapter path
- [ ] Preserve existing call sites in `cmd/web-chat` and other current embedders

### 5. Documentation and migration

- [ ] Update the main webchat framework guide to show the preferred explicit constructor layering
- [ ] Update the user guide to mention the parsed-values adapter vs dependency-injected path
- [ ] Add a dedicated migration guide in `pkg/doc` for embedders moving from `NewRouter/NewServer(parsed, ...)`
- [ ] Relate the new migration guide and touched code files back to this ticket

### 6. Verification and commits

- [ ] Run focused `go test` coverage for `pkg/webchat`
- [ ] Run any doc or embedding tests affected by the API changes
- [ ] Run `docmgr doctor` for `GP-029`
- [ ] Commit in logical increments with diary updates after each completed step
