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

# Webchat Frontend Architecture

This document explains how the React webchat UI is structured, how data flows from the backend into the UI, and how to extend it.

## Source Layout

```
cmd/web-chat/web/src/
  webchat/                # Reusable chat widget + styles
    ChatWidget.tsx        # Top-level component
    components/           # Header / Timeline / Composer / Statusbar
    cards.tsx             # Default entity renderers
    styles/               # theme-default.css + webchat.css
    types.ts              # Public props and types
    parts.ts              # data-part contract
  sem/                    # SEM registry + proto bindings
  ws/                     # wsManager (connect + hydrate + buffer)
  store/                  # Redux slices (app, timeline, errors)
  utils/                  # basePrefix + logger helpers
```

## Runtime Data Flow

1. **ChatWidget** reads `conv_id` from URL and initializes the store.
2. **wsManager** opens `/ws?conv_id=...` and buffers SEM frames.
3. **Hydration** happens via `GET /timeline` (durable snapshot).
4. **SEM registry** decodes frames and dispatches timeline updates.
5. **ChatWidget** renders `timelineSlice` entities using renderer cards.

## Component Hierarchy

```
ChatWidget
  ├─ Header (DefaultHeader or override)
  ├─ Statusbar (DefaultStatusbar or override)
  ├─ Timeline (ChatTimeline)
  │    └─ Renderers (MessageCard, ToolCallCard, etc.)
  └─ Composer (DefaultComposer or override)
```

Renderer mapping is configured in `ChatWidget`:

- `message` → `MessageCard`
- `tool_call` → `ToolCallCard`
- `tool_result` → `ToolResultCard`
- `log` → `LogCard`
- `thinking_mode` → `ThinkingModeCard`
- `planning` → `PlanningCard`
- `default` → `GenericCard`

## State Architecture

Redux Toolkit slices:

- `appSlice`: conv_id, profile, ws status, queue depth
- `timelineSlice`: ordered list of entities (`byId` + `order`)
- `errorsSlice`: UI-visible errors
- `profileApi`: RTK Query for profile endpoints

The timeline state is single-conversation scoped (no per-conv nesting).

## SEM Pipeline

SEM frames are decoded in `sem/registry.ts`:

1. Envelope validation (`sem: true`)
2. Protobuf decode (`fromJson`)
3. Dispatch `addEntity` / `upsertEntity` to `timelineSlice`

Timeline snapshots are converted via `timelineMapper.ts` and upserted on hydrate.

## Theming + Styling

Styling is tokenized:

- `styles/theme-default.css` defines CSS variables (tokens).
- `styles/webchat.css` applies layout and part-based styles.
- DOM uses `data-part` + `data-role` + `data-state` for stable hooks.

Public styling hooks:

- `data-pwchat` root attribute
- `data-part="..."` on stable UI regions

Customization options (props):

- `theme`: select theme name
- `themeVars`: override CSS variables
- `partProps`: customize props per part (className/style)
- `components`: override Header/Composer/Statusbar
- `renderers`: override entity renderers
- `unstyled`: opt out of bundled styles

## How to Extend

### Add a new SEM event + UI card

1. Register a handler in `sem/registry.ts`.
2. Emit a timeline entity (`addEntity` / `upsertEntity`).
3. Add a renderer in `webchat/cards.tsx`.
4. Wire it in `ChatWidget` via `renderers`.
5. Add styling via `data-part` tokens.

### Add a new layout component

1. Create a new component under `webchat/components/`.
2. Add it to `ChatWidget` or expose via `components` props.
3. Add `data-part` hooks and CSS tokens.

### Add a new token/part

1. Add the token to `theme-default.css`.
2. Consume it in `webchat.css`.
3. Use `data-part` attributes for selectors.

## Related Docs

- [Webchat Frontend Integration](webchat-frontend-integration.md)
- [SEM and UI](webchat-sem-and-ui.md)
- [Webchat User Guide](webchat-user-guide.md)
