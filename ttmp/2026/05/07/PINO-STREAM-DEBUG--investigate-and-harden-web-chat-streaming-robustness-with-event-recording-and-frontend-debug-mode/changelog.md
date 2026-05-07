# Changelog

## 2026-05-07

- Initial workspace created

## 2026-05-06

- Updated the streaming investigation guide to consume the new Sessionstream observer plan from `SS-OBSERVERS`.
- Updated tasks so backend recording uses Sessionstream Hub `PipelineRecord` and WebSocket `TransportRecord` values when available.
- Added dependency on `SS-WS-RACE` for proving and fixing reload-during-streaming subscribe/hydration races.

## 2026-05-07

- Noted that Sessionstream observer APIs (`SS-OBSERVERS`) and subscribe hydration buffering (`SS-WS-RACE`) have landed.
- Updated the implementation guide with the corrected reconnect trace that Pinocchio should now consume and verify.
