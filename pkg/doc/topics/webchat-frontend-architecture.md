---
Title: Webchat Frontend Architecture
Slug: webchat-frontend-architecture
Short: Component, state, and data-flow architecture for the React webchat UI.
Topics:
- webchat
- frontend
- architecture
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Source Layout

```
cmd/web-chat/web/src/
  webchat/
    ChatWidget.tsx
    components/
    cards.tsx
    styles/
    types.ts
    parts.ts
  sem/
  ws/
  store/
  utils/
```

## Runtime Data Flow

1. `ChatWidget` initializes conversation state from URL or generated ID.
2. `wsManager` opens `/ws?conv_id=...`.
3. Hydration loads `/api/timeline`.
4. SEM registry decodes websocket events.
5. Timeline slice merges hydration + stream updates.
6. Renderers map entity kinds to cards.

## Component Hierarchy

```
ChatWidget
  Header
  Statusbar
  Timeline
    MessageCard
    ToolCallCard
    ToolResultCard
    LogCard
  Composer
```

## State Slices

- `appSlice`: connection status, profile, queue signals
- `timelineSlice`: timeline entities (`byId` + `order`)
- `errorsSlice`: user-visible errors
- `profileApi`: profile endpoints when app provides them

## SEM Pipeline

1. Validate envelope (`sem: true`).
2. Decode payload.
3. Dispatch timeline updates (`addEntity`, `upsertEntity`, `rekeyEntity`).
4. Preserve ordering via version/sequence semantics.

## Theming and Extension

- Token source: `styles/theme-default.css`
- Structural CSS: `styles/webchat.css`
- Stable selectors: `data-part`, `data-role`, `data-state`
- Override points: `components`, `renderers`, `themeVars`, `partProps`

## Route Assumptions

This UI assumes canonical backend routes:

- `POST /chat`
- `GET /ws`
- `GET /api/timeline`

When mounted with `--root`, the same relative endpoints are used under that prefix.

## See Also

- [Webchat Frontend Integration](webchat-frontend-integration.md)
- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)
- [Webchat User Guide](webchat-user-guide.md)
