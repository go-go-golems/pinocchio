# Web-chat frontend migration checklist

This checklist tracks the intended cleanup path from the current mixed tree to the target one-folder-per-component architecture.

## Accepted decisions

- [x] Production chat runtime is provider-backed `ChatProvider`.
- [x] Provider capability demo code is temporary and should be deleted, not promoted.
- [x] Legacy Redux/WebSocket chat code should be deleted after parity, not retained indefinitely.

## Target path map

| Current path | Target path | Status | Notes |
| --- | --- | --- | --- |
| `src/App.tsx` | `src/app/App.tsx` | in progress | Keep root wrapper while imports migrate. |
| `src/chat/provider/ProviderBackedChatWidget.tsx` | `src/features/web-chat/WebChatProviderShell/` | planned | Provider config + profile bridge. |
| `src/chat/provider/ProviderBackedChatWidgetInner.tsx` | `src/features/web-chat/WebChatApp/` | planned | Production chrome/body. |
| `src/chat/provider/projectors/pinocchioProjectors.ts` | `src/features/web-chat/extensions/pinocchio-projectors/` | planned | Split into reasoning/agent/backend-tool projectors. |
| `src/webchat/components/Header.tsx` | `src/features/web-chat/ChatHeader/ChatHeader.tsx` | planned | Add `types.ts`, `index.ts`, story. |
| `src/webchat/components/Statusbar.tsx` | `src/features/web-chat/ChatStatusbar/ChatStatusbar.tsx` | planned | Add stories for status states. |
| `src/webchat/components/Composer.tsx` | `src/features/web-chat/ChatComposer/ChatComposer.tsx` | planned | Add stories for empty/typed/disabled. |
| `src/webchat/components/Timeline.tsx` | `src/features/web-chat/ChatTimeline/ChatTimeline.tsx` | planned | Move sticky-scroll hook nearby or to shared hooks. |
| `src/webchat/cards.tsx` | `src/features/web-chat/cards/*/` | planned | One folder per card renderer. |
| `src/webchat/ProviderDemoPage.tsx` | delete | planned | Delete after replacement provider/tool/widget coverage exists. |
| `src/chat/provider/ProviderMultiDemoPage.tsx` | test-only harness or delete | planned | Do not keep as production app surface. |
| `src/webchat/ChatWidget.tsx` | delete after parity | planned | Legacy Redux/WebSocket runtime. |
| `src/ws/wsManager.ts` | delete after parity | planned | Legacy singleton transport. |
| `src/ws/timelineEvents.ts` | delete after parity | planned | Replace useful coverage with provider projector tests. |
| `src/ws/timelineSnapshot.ts` | delete after parity | planned | Replace useful coverage with provider/projector tests. |
| `src/webchat/rendererRegistry.ts` | explicit renderer factory | planned | Remove global registration. |
| `src/webchat/timelinePropsRegistry.ts` | projector/renderer-local adapters | planned | Remove global registration. |
| `src/chatapp/pb` | `src/generated/chatapp` | investigate | Confirm Buf/Vite import impact first. |

## Parity gate before deleting legacy chat

- [ ] Session creation and URL/session-id persistence.
- [ ] Profile loading and switching.
- [ ] WebSocket connect/reconnect/snapshot hydration.
- [ ] Message sending and run status transitions.
- [ ] Reasoning/thinking rendering.
- [ ] Backend tool call/result rendering.
- [ ] Typed widget rendering.
- [ ] Frontend tool path, if retained after demo deletion.
- [ ] Export menu with provider session id.
- [ ] Stream debug panel or explicit dev-only replacement.

## Demo deletion gate

- [ ] Remove production imports of `WebChatProviderCapabilities`.
- [ ] Delete `demo.capability_card`.
- [ ] Delete demo tools `browser.get_page_context` and `browser.confirm_action`.
- [ ] Remove `?providerDemo=1` or move equivalent coverage to tests.
- [ ] Replace `run the capabilities demo` smoke with production-relevant coverage.
