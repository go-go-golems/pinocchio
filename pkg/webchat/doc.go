// Package webchat provides conversation lifecycle primitives plus optional UI/API routing helpers.
//
// Ownership model:
//   - Applications own transport routes such as /chat and /ws.
//   - Package helpers (UIHandler/APIHandler/Mount/Handler) are optional utilities for embedding static UI
//     and core APIs (timeline/debug), not the canonical transport composition layer.
//
// Recommended setup:
//   - Build a Server with NewServer and a RuntimeComposer.
//   - Create app-owned /chat and /ws handlers via NewChatHTTPHandler/NewWSHTTPHandler with ChatService/StreamHub.
//   - Mount NewTimelineHTTPHandler at /api/timeline.
//   - Mount Router/API/UI helpers where needed (for example under /api/ and /).
package webchat
