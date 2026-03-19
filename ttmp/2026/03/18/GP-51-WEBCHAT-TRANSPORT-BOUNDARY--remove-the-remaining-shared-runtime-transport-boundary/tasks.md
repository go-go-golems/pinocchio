# Tasks

## Slice 1: Inventory the remaining shared boundary

- [ ] Identify every remaining user of `pkg/inference/runtime.ProfileRuntime`.
- [ ] Identify every remaining user of `webhttp.ResolvedConversationRequest`.
- [ ] Identify which fields are truly required by shared conversation lifecycle code and which are only app-level transport residue.
- [ ] Record the current end-to-end flow from HTTP request to `RuntimeBuilder.Compose(...)`.

## Slice 2: Choose the replacement boundary

- [ ] Evaluate a narrowed DTO approach:
  - keep a shared request type
  - remove prompt/middlewares/tools from it
  - leave only identity and engine-composition hooks
- [ ] Evaluate a compose-capable interface approach:
  - app returns a local plan plus a composer/closure/interface
  - shared webchat stops receiving app runtime payload as data
- [ ] Decide whether `ConversationRuntimeRequest` survives or becomes app-local.
- [ ] Write the architecture recommendation in the design doc with tradeoffs.

## Slice 3: Define the target API shape

- [ ] Propose the final shared boundary types or interfaces.
- [ ] Show the replacement for `ProfileRuntime`.
- [ ] Show the replacement for `ResolvedConversationRequest`.
- [ ] Show how `ChatService` and `StreamHub` would consume the new shape.
- [ ] Show how `cmd/web-chat`, CoinVault, and Temporal would adapt.

## Slice 4: Migration plan

- [ ] Define a hard-cut migration order for Pinocchio, CoinVault, and Temporal.
- [ ] Identify the smallest first implementation slice.
- [ ] Document validation commands and test coverage needed at each step.

## Ticket bookkeeping

- [ ] Keep the diary updated as analysis evolves.
- [ ] Update the changelog when the architecture recommendation changes.
