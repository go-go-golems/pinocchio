# Changelog

## 2026-08-19

- Initial workspace created


## 2026-08-19

Rolled out --export-mode sqlite across go-go-golems repos: opened 8 PRs (flowkit#8 add CLI, go-go-wm#3 migrate glazed v1.4.3 + cmd path, remarquee#25, pinocchio#201, judgekit#1, go-template#3, ragopt#2, infra-tooling#33 template). Version-gated: only glazed v1.4.0+ repos changed; 32 v1.3.x repos left untouched (flag-only change would break their builds via glazed v1.4 API removals).

### Related Files

- /home/manuel/code/wesen/go-go-golems/go-go-wm/pkg/cmds/query.go — glazed v1.4.3 settings API migration

