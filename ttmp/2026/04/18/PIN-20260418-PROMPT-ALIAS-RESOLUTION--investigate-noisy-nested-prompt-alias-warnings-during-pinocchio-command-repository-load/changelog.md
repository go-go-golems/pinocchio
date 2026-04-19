# Changelog

## 2026-04-18

- Initial workspace created


## 2026-04-18

Created follow-up ticket for noisy nested prompt alias warnings. Initial analysis shows the warnings come from alias path resolution (prefix + aliasFor => code go go) rather than command load order; commands are inserted before aliases.

### Related Files

- /home/manuel/go/pkg/mod/github.com/go-go-golems/clay@v0.4.3/pkg/repositories/repository.go — Clay resolves alias lookup as alias.Parents + aliasFor and emits the warning
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/glazed/pkg/cmds/loaders/loaders.go — Loader assigns alias parents from the containing directory path
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/prompts/code/go.yaml — Real target command for the nested go aliases
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/prompts/code/go/concise-doc.yaml — Representative nested alias fixture using aliasFor: go under code/go/


## 2026-04-18

Implemented explicit alias-path support. Glazed now parses aliasFor as either a legacy relative slug or a full path, Clay resolves aliases through the shared helper, nested Pinocchio prompt aliases were migrated to full-path form, and pinocchio --help no longer emits the noisy alias-not-found warnings.

### Related Files

- /home/manuel/code/wesen/go-go-golems/clay/pkg/repositories/repository.go — Repository alias lookup now honors explicit full alias paths
- /home/manuel/code/wesen/go-go-golems/clay/pkg/repositories/repository_test.go — Regression test for nested alias mounted under code/go targeting explicit path code go
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/glazed/pkg/cli/cobra.go — Cobra alias resolution now uses the shared resolved target path
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/glazed/pkg/cmds/alias/alias.go — Introduced AliasTarget decoding and shared ResolveAliasedCommandPath semantics
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/glazed/pkg/cmds/alias/alias_test.go — Regression coverage for scalar and path-form aliasFor decoding
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/README.md — README alias docs now mention explicit full command paths for nested aliases
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/prompts/code/go/concise-doc.yaml — Nested alias fixture migrated to explicit aliasFor path form

