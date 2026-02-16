# Mock Data Architecture

This directory defines a layered mock-data system for Storybook and MSW:

- `fixtures/`: canonical static fixture objects grouped by domain.
- `factories/`: deterministic builders for fixture-derived variations.
- `scenarios/`: reusable story contexts composed from factories.
- `msw/`: reusable endpoint handler builders and default handler bundles.

## Layer Contracts

### `fixtures/`

- Store stable baseline examples only.
- Keep fixtures domain-scoped (`conversations`, `turns`, `events`, `timeline`, `anomalies`).
- Do not place story-specific one-off variants here.

### `factories/`

- Build deterministic variants from fixtures.
- Prefer builder APIs (`makeX`, `makeXs`) over ad-hoc story cloning.
- Use deterministic helpers for synthetic id/time/seq behavior when fixture indices wrap.

### `scenarios/`

- Export reusable named contexts for stories (`default`, `empty`, `manyItems`, etc.).
- Keep scenario names aligned with story intent.
- Stories should consume `make*Scenario(...)` rather than rebuilding arrays inline.

### `msw/`

- `createDebugHandlers.ts`: generic handler builder for debug endpoints.
- `defaultHandlers.ts`: fixture-backed defaults plus optional override hooks.
- Story/runtime handler wiring should import these modules, not redefine routes inline.

## Story Authoring Rules

- Prefer `scenarios` first.
- If a needed scenario does not exist, add it under `scenarios/`.
- If scenario composition requires a new data pattern, add/extend a factory.
- Avoid large local arrays in story files.
- Avoid direct `msw` route duplication in stories; use handler helpers from `msw/`.

## Quick Examples

```ts
// Scenario-first story args
import { makeTimelineScenario } from '../mocks/scenarios';
export const Default = { args: makeTimelineScenario('default').args };
```

```ts
// Centralized handlers
import { createDefaultDebugHandlers } from '../mocks/msw/defaultHandlers';
export const WithMSW = { parameters: { msw: { handlers: createDefaultDebugHandlers() } } };
```
