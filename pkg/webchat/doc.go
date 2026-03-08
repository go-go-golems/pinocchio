// Package webchat provides conversation lifecycle primitives plus optional UI/API routing helpers.
//
// Ownership model:
//   - Applications own transport routes such as /chat and /ws.
//   - Package helpers (UIHandler/APIHandler) expose the embedded UI and core APIs (timeline/debug),
//     but applications still own HTTP route composition.
//
// Recommended setup:
//   - Build a Server with NewServer and a RuntimeBuilder.
//   - Create app-owned /chat and /ws handlers via webhttp.NewChatHandler/webhttp.NewWSHandler with ChatService/StreamHub.
//   - Mount webhttp.NewTimelineHandler at /api/timeline.
//   - Mount APIHandler/UIHandler where needed (for example under /api/ and /).
package webchat
