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
| `src/chat/provider/ProviderBackedChatWidget.tsx` | `src/features/web-chat/WebChatProviderShell/` | done | Provider config + profile bridge now live in `WebChatProviderShell.tsx`; old path re-exports through compatibility index. |
| `src/chat/provider/ProviderBackedChatWidgetInner.tsx` | `src/features/web-chat/WebChatApp/` | done | Production chrome/body now live in `WebChatApp.tsx`. |
| `src/chat/provider/projectors/pinocchioProjectors.ts` | `src/features/web-chat/extensions/pinocchio-projectors/` | moved | Split into reasoning/agent/backend-tool files later in Phase 9. |
| `src/webchat/components/Header.tsx` | `src/features/web-chat/ChatHeader/ChatHeader.tsx` | done | Added `types.ts`, `index.ts`, and focused stories. |
| `src/webchat/components/Statusbar.tsx` | `src/features/web-chat/ChatStatusbar/ChatStatusbar.tsx` | done | Added connected/disconnected/error/export-visible stories. |
| `src/webchat/components/Composer.tsx` | `src/features/web-chat/ChatComposer/ChatComposer.tsx` | done | Added empty/typed/disabled/long-text stories. |
| `src/webchat/components/Timeline.tsx` | `src/features/web-chat/ChatTimeline/ChatTimeline.tsx` | done | Moved sticky-scroll hook into `ChatTimeline/` and added focused timeline stories. |
| `src/webchat/cards.tsx` | `src/features/web-chat/cards/*/` | in progress | Card renderers now live in one folder per card; old file is a compatibility barrel until imports move. |
| `src/webchat/ProviderDemoPage.tsx` | delete | done | Provider demo route and capability registration were deleted in Phase 5. |
| `src/chat/provider/ProviderMultiDemoPage.tsx` | `src/features/web-chat/demos/ProviderMultiDemo/` then test-only harness or delete | route removed | Not reachable from production route parsing; remaining demo files are not exported through app routes. |
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

- [x] Remove production imports of `WebChatProviderCapabilities`.
- [x] Delete `demo.capability_card`.
- [x] Delete demo tools `browser.get_page_context` and `browser.confirm_action`.
- [x] Remove `?providerDemo=1` and route removed provider demo flags to production chat.
- [x] Replace `run the capabilities demo` coverage with production route tests, main web-chat smoke, frontend-tool endpoint tests, and focused card stories.
