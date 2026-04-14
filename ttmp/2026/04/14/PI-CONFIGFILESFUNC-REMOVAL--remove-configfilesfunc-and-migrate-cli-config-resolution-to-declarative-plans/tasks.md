# Tasks

## DONE

- [x] Remove `ConfigFilesFunc` from `glazed/pkg/cli.CobraParserConfig`
- [x] Add a single explicit plan-based config-loading hook to `CobraParserConfig`
- [x] Stop implicit config discovery from `AppName` in the default CobraParser middleware chain
- [x] Remove `ConfigPath` from `CobraParserConfig` and migrate remaining callers in this workspace
- [x] Migrate Pinocchio `web-chat`, `simple-chat`, and `simple-chat-agent` off the current no-op `ConfigFilesFunc` suppression pattern
- [x] Migrate `glazed/cmd/examples/config-overlay` to declarative config plans
- [x] Migrate `glazed/cmd/examples/overlay-override` to declarative config plans
- [x] Update current docs/examples that still presented the old path-list config-loading API
- [x] Audit `pkg/appconfig` usage and choose deletion over modernization for this workspace cleanup
- [x] Remove `pkg/appconfig` and its Glazed examples in the same change instead of preserving a compatibility facade
- [x] Remove `glazed/pkg/config/ResolveAppConfigPath(...)` and migrate remaining workspace callers to explicit plans
- [x] Update active Glazed docs to teach `sources.FromConfigPlan(...)` / `sources.FromConfigPlanBuilder(...)` as direct plan-loading middlewares alongside `ConfigPlanBuilder` and `FromResolvedFiles(...)`

## FOLLOW-UPS

- [x] Add `sources.FromConfigPlan(...)` / `sources.FromConfigPlanBuilder(...)` as high-level middleware wrappers over `FromResolvedFiles(...)`, then update CobraParser to use that middleware internally instead of resolving plans directly in `pkg/cli/cobra-parser.go`
