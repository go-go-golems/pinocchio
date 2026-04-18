# Tasks

## Investigation baseline

- [x] Capture the warning shape and identify representative alias files
- [x] Trace the loader / repository path that emits the warning
- [x] Determine whether commands are inserted before aliases
- [x] Write up the initial root-cause note

## Follow-up implementation planning

- [ ] Decide the intended contract for nested alias files (`same-prefix` vs `parent-relative` aliasing)
- [ ] Add a focused regression test covering `code/go.yaml` + `code/go/concise-doc.yaml`
- [ ] Choose between fixing prompt layout, shared alias resolution semantics, or both
- [ ] Implement the chosen fix in the appropriate repo(s)
- [ ] Re-run `pinocchio --help` / startup smoke and confirm the warnings are gone or intentionally preserved

