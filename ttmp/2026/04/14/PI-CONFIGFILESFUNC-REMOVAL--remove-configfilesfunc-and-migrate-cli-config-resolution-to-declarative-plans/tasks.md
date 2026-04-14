# Tasks

## TODO

- [ ] Remove `ConfigFilesFunc` from `glazed/pkg/cli.CobraParserConfig`
- [ ] Add a single explicit plan-based config-loading hook to `CobraParserConfig`
- [ ] Stop implicit config discovery from `AppName` in the default CobraParser middleware chain
- [ ] Decide whether `ConfigPath` should be removed in the same change; if yes, remove it from `CobraParserConfig` and migrate remaining callers
- [ ] Migrate Pinocchio `web-chat`, `simple-chat`, and `simple-chat-agent` off the current no-op `ConfigFilesFunc` suppression pattern
- [ ] Migrate `glazed/cmd/examples/config-overlay` to declarative config plans
- [ ] Migrate `glazed/cmd/examples/overlay-override` to declarative config plans
- [ ] Update docs/examples that still present the old path-list config-loading API
- [ ] Audit `pkg/appconfig` in `corporate-headquarters/prescribe` and decide whether to migrate that caller off `pkg/appconfig` instead of modernizing the package
- [ ] If `pkg/appconfig` has no remaining compelling production consumers after migration, open or fold in a follow-up to deprecate/remove it
