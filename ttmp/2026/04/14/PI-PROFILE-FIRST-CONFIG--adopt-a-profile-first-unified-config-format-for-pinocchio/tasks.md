# Tasks

## DONE

- [x] Create the ticket workspace and establish the document set for research and design
- [x] Map the current Pinocchio/Geppetto/Glazed architecture relevant to config plans, profile selection, profile registries, and runtime switching
- [x] Write an evidence-based current-state analysis document with file references, architecture mapping, and identified design pressures
- [x] Write a primary design document for a profile-first unified config format
- [x] Write a detailed implementation guide for a new intern, including phased work, pseudocode, file targets, risks, and validation strategy
- [x] Maintain an investigation diary for the design work
- [x] Relate the key code and documentation files to the new ticket documents
- [x] Validate the ticket documentation with `docmgr doctor`
- [x] Upload a bundled PDF of the new ticket docs to reMarkable and verify the remote listing

## IMPLEMENTATION BACKLOG

### Phase 1 — typed document foundation

- [ ] Add `pinocchio/pkg/configdoc` with typed structs for `Document`, `AppBlock`, `ProfileBlock`, and `InlineProfile`
- [ ] Add strict YAML decoding for the new format only; reject unknown top-level legacy keys such as `ai-chat` and `profile-settings`
- [ ] Validate config-document slugs and profile stack references using existing `engineprofiles` slug rules
- [ ] Decide and encode the canonical new local filename policy (recommended: `.pinocchio.yml`) with no runtime compatibility alias for `.pinocchio-profile.yml`
- [ ] Add focused unit tests for decode/validation failures and valid minimal documents

### Phase 2 — layered document merge

- [ ] Implement merge logic for layered unified config documents loaded from the existing config-plan path
- [ ] Define scalar merge rules (`profile.active` last-writer-wins, `profile.registries` last-writer-wins)
- [ ] Implement `app.repositories` merge semantics: append in layer order, dedupe, preserve stable order
- [ ] Implement same-slug inline profile merges across layers, including explicit rules for `stack`, `inference_settings`, and `extensions`
- [ ] Add provenance/explain data for merged app/profile/profile entries so later debug output can still explain which layer won
- [ ] Add merge tests for user/repo/cwd/explicit layering, including repository accumulation and same-slug profile overrides

### Phase 3 — inline profiles as registry input

- [ ] Add an inline-profile-to-registry adapter that converts merged `profiles` into a synthetic `engineprofiles.EngineProfileRegistry`
- [ ] Compose imported registries from `profile.registries` with the synthetic inline registry into one final registry view
- [ ] Define and test same-slug precedence between imported and inline profiles (recommended: inline wins)
- [ ] Preserve existing `stack` resolution semantics across inline and imported profiles
- [ ] Add focused tests for inline-only, imported-only, and mixed inline/imported resolution paths

### Phase 4 — document-first bootstrap

- [ ] Add unified config resolution helpers in Pinocchio that resolve files, load/merge the effective document, and expose app/profile/profile-catalog results
- [ ] Replace the current mapper-first runtime-config path with a document-first bootstrap path for profile selection and engine settings
- [ ] Determine the minimal Geppetto bootstrap seam needed to consume document-derived profile state without reintroducing path-centric helpers
- [ ] Preserve the current base-plus-selected-profile runtime model while switching the config source model underneath it
- [ ] Add focused bootstrap tests for resolved files + unified document + selected profile behavior

### Phase 5 — fold app settings into the unified document

- [ ] Move repository loading fully into `app.repositories` in the unified config document
- [ ] Remove or collapse the separate repository-only loader path in `pinocchio/pkg/cmds/profilebootstrap/repositories.go`
- [ ] Update `cmd/pinocchio/main.go` and any other repository consumers to read from the unified document path
- [ ] Add tests proving repository lists merge across config layers as designed

### Phase 6 — runtime consumer migration

- [ ] Update `pinocchio/pkg/cmds/profilebootstrap/*` to consume the unified document path
- [ ] Update `pinocchio/cmd/pinocchio/cmds/js.go` to use unified config + inline/imported profile resolution
- [ ] Update `pinocchio/cmd/web-chat/main.go` to use unified config + composed registry resolution
- [ ] Verify runtime profile switching still preserves a non-profile baseline and rebuilds from base rather than prior merged state
- [ ] Revalidate any remaining Pinocchio command/example paths that currently assume top-level runtime sections in config files

### Phase 7 — breaking-change handling and migration tooling

- [ ] Fail loudly on old config shape (`ai-chat`, `profile-settings`, legacy local filename) instead of supporting compatibility parsing
- [ ] Write a user-facing migration guide from old top-level runtime config to the new `app` / `profile` / `profiles` format
- [ ] Investigate whether Pinocchio should add a dedicated migration verb such as `pinocchio config migrate`
- [ ] If worthwhile, implement a one-shot migration command that rewrites old config into the new format without keeping runtime compatibility code

### Phase 8 — tests, docs, and rollout

- [ ] Add end-to-end tests for repo/cwd/explicit precedence under the new format
- [ ] Add tests for inline profile selection, imported registry selection, and inline override of imported same-slug profiles
- [ ] Add failure tests for old-format config files and old local filenames
- [ ] Update Pinocchio docs and examples to teach only the unified `app` / `profile` / `profiles` model
- [ ] Update Geppetto migration/help docs where they still implicitly assume top-level runtime config in app config files
- [ ] Upload refreshed implementation docs / migration docs to reMarkable once the first implementation tranche lands
