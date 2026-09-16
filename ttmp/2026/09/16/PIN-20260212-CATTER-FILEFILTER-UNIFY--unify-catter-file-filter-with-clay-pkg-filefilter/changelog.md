# Changelog

## 2026-09-16

- Initial workspace created

## 2026-09-16

Wrote intern-ready analysis/design/implementation guide for unifying catter file filter with clay pkg/filefilter: evidence that the copies are identical except the constructor name, conclusion that plain replacement works, phased plan with upstream segment-matching fix and include-dirs design

### Related Files

- /Users/manuel.odendahl/code/go-go-golems/pinocchio/pkg/filefilter/section.go — Renamed constructor NewFileFilterParameterLayer is the only code difference to clay layer.go

## 2026-09-16

Design amendment per user request: replaced exact-segment matching with glob-per-segment matching (filepath.Match, stdlib only) for DefaultExcludedDirs/ExcludeDirs and the new IncludeDirs; plain names stay exact matches, substring behavior remains opt-in via *build* globs; added malformed-pattern error policy and Windows separator notes to risks

### Related Files

- /Users/manuel.odendahl/code/go-go-golems/pinocchio/ttmp/2026/09/16/PIN-20260212-CATTER-FILEFILTER-UNIFY--unify-catter-file-filter-with-clay-pkg-filefilter/analysis/01-catter-file-filter-unification-analysis-design-and-implementation-guide.md — Design sections 5.2, 6, 7, 8.2 amended to glob semantics

## 2026-09-16

Implementation: clay commit 4350e72 (glob-per-segment dir matching, include-dirs, Validate; tests + doc), pinocchio commit 91ac79ef (catter switched to clay filefilter, local copy deleted). Phase 3 integration validated end-to-end via temporary replace (builder-api included, build excluded, include-dirs wins, bad glob rejected); replace not committed. Remaining: clay release + pinocchio go.mod bump

### Related Files

- /Users/manuel.odendahl/code/go-go-golems/clay/pkg/filefilter/filefilter.go — glob-per-segment isExcludedDir, Validate, IncludeDirs
- /Users/manuel.odendahl/code/go-go-golems/pinocchio/cmd/pinocchio/cmds/catter/cmds/print.go — import switch to clay filefilter

## 2026-09-16

Clay PR opened: go-go-golems/clay#149 (branch task/filefilter-glob-segment-matching pushed to tulip fork), title/body follow create-pull-request prompt conventions; awaiting review/merge, then tag + pinocchio bump
