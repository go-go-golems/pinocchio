# Web-chat styles

The web-chat stylesheet is scoped under `[data-pwchat]` and public `data-part` names. Import `index.css` once from the provider shell or Storybook preview.

## Files

- `themes/default.css` — default theme tokens (`--pwchat-*`).
- `root.css` — root scoping, font, box sizing, fullscreen grid.
- `layout.css` — main scroll area, timeline flow, turn/bubble/content layout.
- `header.css` — header shell and title.
- `statusbar.css` — statusbar, pills, pill buttons, and profile select.
- `timeline.css` — error panel and error item layout.
- `cards.css` — card shell, markdown, mono, toolbar, generic buttons, and card-specific primitives.
- `composer.css` — composer layout, input, actions, and send button.

## Public parts

The stable public parts are the values accepted by `ChatPart` in `src/features/web-chat/types.ts`: `root`, `header`, `timeline`, `composer`, `statusbar`, `turn`, `bubble`, `content`, `composer-input`, `composer-actions`, and `send-button`.

Additional internal parts such as `card`, `card-header`, `pill`, `toolbar`, `mono`, `markdown`, and `error-panel` are intentionally styled here but may change as individual cards evolve.

## Theming

Themes override `--pwchat-*` variables on `[data-pwchat][data-theme="..."]`. The default shell sets `data-theme="default"` unless the unstyled variant is requested.
