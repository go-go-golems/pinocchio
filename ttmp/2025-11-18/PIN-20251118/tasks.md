# Tasks

## TODO


- [ ] Replace clay.InitViper/logging.InitLoggerFromViper usages with InitGlazed + CobraParserConfig + SetupLoggingFromParsedLayers (cmd/pinocchio, agents, examples).
- [ ] Adopt LoadParametersFromFiles/UpdateFromEnv for pinocchio config (incl. layered config format or mapper) and update README/docs.
- [ ] Fix profile middleware chain so --profile/PINOCCHIO_PROFILE actually load the requested profile before ai-chat defaults are applied.
- [ ] Remove remaining GatherFlagsFromViper (helpers.ParseGeppettoLayers, catter, etc.) and add regression tests covering profile/env precedence.
- [ ] Implement resolveProfileSettings helper that pre-parses command/profile layers via mini middleware chain using existing config resolver/env/CLI middlewares.
- [ ] Wire resolveProfileSettings into geppetto/pkg/layers.GetCobraCommandGeppettoMiddlewares so profile middleware uses resolved profile/profile-file.
- [ ] Update helpers.ParseGeppettoLayers (and sample commands) to use the shared profile-resolution logic instead of GatherFlagsFromViper.
- [ ] Add regression tests (env, flag, default) + docs describing profile precedence and usage.
- [ ] Upstream the field-ignoring config mapper (used to skip repositories) into a reusable Glazed helper so other apps can drop non-layer keys cleanly.
