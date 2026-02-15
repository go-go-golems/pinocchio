// Package webchat provides conversation lifecycle primitives plus optional UI/API routing helpers.
//
// Ownership model:
//   - Applications own transport routes such as /chat and /ws.
//   - Package helpers (UIHandler/APIHandler/Mount/Handler) are optional utilities for embedding static UI
//     and core APIs (timeline/debug), not the canonical transport composition layer.
//
// Recommended setup:
//   - Build a Router with a RuntimeComposer.
//   - Create app-owned /chat and /ws handlers via NewChatHandler/NewWSHandler and ConversationService.
//   - Mount Router API/UI helpers where needed (for example under /api/ and /).
package webchat
