# Changelog

## 2026-03-18

- Initial workspace created.
- Added the first analysis/design ticket for removing the remaining shared runtime transport boundary in Pinocchio webchat.
- Scoped the ticket around:
  - `pkg/inference/runtime.ProfileRuntime`
  - `pkg/webchat/http.ResolvedConversationRequest`
  - `pkg/inference/runtime.ConversationRuntimeRequest`
- Captured the core design question: whether the last shared boundary should become a narrower DTO or a compose-capable interface owned at the app layer.
