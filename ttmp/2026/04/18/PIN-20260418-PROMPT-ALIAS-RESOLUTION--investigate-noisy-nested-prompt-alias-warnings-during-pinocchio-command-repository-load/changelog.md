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

