# Tasks

## Implementation

- [x] Run `devctl help --all` and read the relevant user, scripting, plugin-authoring, and profiles help pages.
- [x] Inspect the existing `cmd/web-chat` devctl plugin and Pinocchio web-chat profile flags.
- [x] Add repository-root `.devctl.yaml` support for launching web-chat from the Pinocchio root.
- [x] Update `cmd/web-chat/.devctl.yaml` to use devctl profiles as well.
- [x] Update the web-chat devctl plugin to work from either repo root or `cmd/web-chat`.
- [x] Add devctl profiles for normal web-chat and provider-observability web-chat modes.
- [x] Thread profile-related environment through the plugin into web-chat CLI flags.
- [x] Replace interactive-shell service wrappers with `bash --noprofile --norc` wrappers.
- [x] Fix plugin step result shapes for dry-run `build.run` / `prepare.run`.
- [x] Validate with `devctl profiles list`, `devctl plugins list`, `devctl plan --dry-run`, and `devctl up --dry-run`.
- [x] Run a real `devctl up --force`, check status, fetch `/api/chat/profiles`, then run `devctl down`.

## Documentation and delivery

- [x] Create the `PINO-DEVCTL-WEBCHAT` ticket under `pinocchio/ttmp`.
- [x] Write the intern-facing analysis/design/implementation guide.
- [x] Keep an implementation diary.
- [x] Relate implementation files to the design document with `docmgr doc relate`.
- [x] Upload the guide bundle to reMarkable.
- [x] Commit implementation and ticket docs.
