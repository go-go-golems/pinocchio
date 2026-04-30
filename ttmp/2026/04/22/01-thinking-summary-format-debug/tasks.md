# Tasks

- [x] Add temporary backend logging around reasoning-summary / thinking delta emission.
- [x] Capture websocket frames in the browser and compare them to backend logs.
- [x] Launch `cmd/web-chat` with a SQLite timeline/hydration DB and generate one reasoning-heavy turn.
- [x] Inspect the persisted snapshot / database and confirm whether newlines survive to storage.
- [x] Decide whether the flattening is happening in transport, persistence, or rendering.
- [x] Trace the live Geppetto provider path and determine whether `cmd/web-chat` still uses the legacy SEM translator.
- [x] Patch the OpenAI Responses reasoning-summary accumulator to preserve sentence/markdown boundaries.
- [x] Add regression tests in Geppetto and rerun the relevant web-chat reasoning tests.
- [x] Re-run the live `cmd/web-chat` server against patched Geppetto and verify the repaired summary in the UI and persisted session snapshot.
- [x] Remove the temporary reasoning debug logging from `cmd/web-chat/reasoning_chat_feature.go`.
