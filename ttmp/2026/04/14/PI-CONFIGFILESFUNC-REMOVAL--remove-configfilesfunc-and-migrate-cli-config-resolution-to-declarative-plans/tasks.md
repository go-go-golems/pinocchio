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
- [x] Remove `InitViper(...)` from the local Clay module and audit remaining active `corporate-headquarters` Go call sites that still depend on it
- [x] Sweep the active legacy `corporate-headquarters` programs off removed worktree startup APIs (`clay.InitViper`, `logging.InitLoggerFromViper`, `logging.InitViper`) without trying to fully modernize or revalidate those programs

## FOLLOW-UPS

- [x] Add `sources.FromConfigPlan(...)` / `sources.FromConfigPlanBuilder(...)` as high-level middleware wrappers over `FromResolvedFiles(...)`, then update CobraParser to use that middleware internally instead of resolving plans directly in `pkg/cli/cobra-parser.go`
- [x] Delete the duplicated legacy Geppetto Cobra middleware builders in `geppetto/pkg/sections/sections.go` and `geppetto/pkg/sections/profile_sections.go` after migrating any remaining callers to `geppetto/pkg/cli/bootstrap`
- [x] Remove pinocchio-specific config/profile policy helpers from Geppetto `pkg/sections` so that package only owns section construction, not app policy
- [x] Delete `pinocchio/pkg/cmds/helpers/parse-helpers.go` and migrate `cmd/examples/simple-chat` off the `GeppettoLayersHelper` / `UseViper` compatibility path
- [ ] Collapse duplicated config middleware assembly in `geppetto/pkg/cli/bootstrap/{profile_selection,engine_settings,inference_debug}.go` into one shared resolved-files helper and remove the dead `FromFiles(...)` fallback branch
- [ ] Migrate `pinocchio/cmd/pinocchio/cmds/js.go` off `ResolveCLIConfigFiles(...) + FromFiles(...)` to a resolved-files or direct plan-middleware path, and extract the profile-registry-chain builder if that logic still needs to be shared
- [ ] Evaluate whether `pinocchio/cmd/pinocchio/main.go` repository-loading should stop manually parsing YAML and instead use a typed helper over the same plan-based config path
- [ ] Shrink or delete thin Pinocchio helper re-export layers in `pkg/cmds/helpers/*` once callers use `profilebootstrap` directly
