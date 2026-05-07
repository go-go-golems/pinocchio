---
Title: Implementation Diary
Ticket: PINO-DEVCTL-WEBCHAT
Status: active
Topics:
    - devctl
    - web-chat
    - profiles
    - developer-experience
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Chronological diary for adding devctl support to launch Pinocchio web-chat with devctl profiles and Pinocchio profile-aware flags.
LastUpdated: 2026-05-07T17:25:00-04:00
WhatFor: Record commands, decisions, implementation details, validation results, and follow-up notes for PINO-DEVCTL-WEBCHAT.
WhenToUse: Use when reviewing or resuming the devctl web-chat launcher work.
---

# Implementation Diary

## Step 1: Read devctl help and identify the right integration shape

### Prompt Context

**User prompt (verbatim):** "Add `devctl` support to pinocchio to launch web-chat, using the new profiles feature. Run `devctl help --all` and figure it out. Create a new docmgr ticket and Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Add repository-level devctl support for launching Pinocchio web-chat, use devctl profiles for launch modes, keep support for the existing `cmd/web-chat` devctl workflow, document the system deeply, and upload the ticket guide to reMarkable.

**Inferred user intent:** Make `devctl up` a reliable onboarding command for web-chat while preserving Pinocchio's new profile-aware runtime selection.

### What I did

- Ran `devctl help --all`.
- Read:
  - `devctl help user-guide`
  - `devctl help scripting-guide`
  - `devctl help plugin-authoring`
  - `devctl help profiles-guide`
- Confirmed the required plugin pattern:
  - handshake must be the first stdout frame;
  - stdout must remain NDJSON-only;
  - logs must go to stderr;
  - `launch.plan` returns service specs and devctl supervises them;
  - profiles select plugin IDs and overlay env vars.

### What I learned

- `devctl` profiles are selection/configuration overlays, not separate pipelines.
- Profile env is a good place to select web-chat launch modes such as normal mode vs provider-observability mode.
- The existing `cmd/web-chat/plugins/webchat.py` was already close to the right shape, but it was scoped to `cmd/web-chat` and did not use devctl profiles.

## Step 2: Create the ticket and fix docmgr root resolution

### What I did

- Created `PINO-DEVCTL-WEBCHAT` under the intended Pinocchio ticket root with an absolute `--root` path:
  - `/home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp`
- Added:
  - this diary;
  - the design/implementation guide;
  - task list updates.

### What didn't work

- An initial `docmgr ticket create-ticket` without an absolute root resolved through the workspace `.ttmp.yaml` to an unrelated sibling docs root.
- I removed that accidental sibling ticket directory and recreated the ticket under `pinocchio/ttmp` using an absolute root path.

### Lesson

When this workspace has a parent `.ttmp.yaml`, use an absolute `--root /.../pinocchio/ttmp` for Pinocchio-specific docmgr operations.

## Step 3: Implement repository-root devctl support

### What I did

- Added root `.devctl.yaml` so developers can run devctl from the Pinocchio repository root.
- Updated `cmd/web-chat/.devctl.yaml` so the existing subdirectory workflow also uses profiles.
- Added two profiles:
  - `web-chat`: normal web-chat launch with debug API enabled and Geppetto trace off;
  - `web-chat-observe`: provider-observability launch with `--geppetto-trace-level provider`.
- Kept the plugin ID stable: `pinocchio-webchat`.

### Why

The repo root is where a new contributor is likely to start, but the web-chat app still lives under `cmd/web-chat`. The plugin now supports both working directories.

## Step 4: Harden and extend the plugin

### What I changed

- Updated `cmd/web-chat/plugins/webchat.py` to detect whether `ctx.repo_root` is:
  - the Pinocchio repository root; or
  - `cmd/web-chat`.
- Added profile-aware environment variables:
  - `PINOCCHIO_WEBCHAT_PROFILE`
  - `PINOCCHIO_WEBCHAT_PROFILE_REGISTRIES`
  - `PINOCCHIO_WEBCHAT_TRACE_LEVEL`
  - `PINOCCHIO_WEBCHAT_DEBUG_API`
  - `PINOCCHIO_WEBCHAT_ROOT`
  - `PINOCCHIO_WEBCHAT_BACKEND_PORT`
  - `PINOCCHIO_WEBCHAT_VITE_PORT`
- Threaded those values into config patches and launch arguments.
- Replaced `bash -lc` service wrappers with `bash --noprofile --norc -lc` to avoid interactive shell startup noise.
- Added `command.run` to the advertised operations so the existing dynamic commands are discoverable.
- Fixed `build.run` and `prepare.run` step output to use `ok: true`, which is what devctl expects in dry-run output.

### Validation commands

```bash
cd pinocchio
python3 -m py_compile cmd/web-chat/plugins/webchat.py
devctl profiles list
devctl plugins list
devctl plan --dry-run
devctl plan --profile web-chat-observe --dry-run
devctl up --dry-run --timeout 60s
```

### Real launch smoke

```bash
cd pinocchio
devctl up --force --timeout 180s
devctl status --tail-lines 5
python3 - <<'PY'
import urllib.request
with urllib.request.urlopen('http://127.0.0.1:8092/api/chat/profiles', timeout=5) as r:
    print(r.status, r.read().decode()[:300])
PY
devctl down
```

Result:

- `devctl up` started `backend` and `vite`.
- `devctl status` showed both services alive.
- `/api/chat/profiles` returned HTTP 200 and profile JSON.
- `devctl down` stopped the `web-chat` profile successfully.

## Step 5: Write the intern guide and prepare for reMarkable upload

### What I did

- Wrote the design/implementation guide as a long-form onboarding document.
- Included:
  - system overview;
  - devctl protocol notes;
  - profile selection explanation;
  - service launch diagrams;
  - CLI/API references;
  - file references;
  - pseudocode;
  - validation playbook;
  - troubleshooting notes.

### Next

- Relate docs to implementation files with `docmgr doc relate`.
- Upload the guide bundle to reMarkable.
- Commit implementation and docs.

## Step 6: Upload guide bundle to reMarkable

### What I did

- Ran `remarquee status` successfully.
- Dry-ran a bundle upload with:
  - the design/implementation guide;
  - the implementation diary.
- Uploaded the bundle to:
  - `/ai/2026/05/07/PINO-DEVCTL-WEBCHAT/PINO-DEVCTL-WEBCHAT devctl Web Chat Launch Guide.pdf`
- Verified the remote listing.

### Commands

```bash
remarquee upload bundle --dry-run \
  <guide.md> <diary.md> \
  --name "PINO-DEVCTL-WEBCHAT devctl Web Chat Launch Guide" \
  --remote-dir "/ai/2026/05/07/PINO-DEVCTL-WEBCHAT" \
  --toc-depth 2

remarquee upload bundle \
  <guide.md> <diary.md> \
  --name "PINO-DEVCTL-WEBCHAT devctl Web Chat Launch Guide" \
  --remote-dir "/ai/2026/05/07/PINO-DEVCTL-WEBCHAT" \
  --toc-depth 2

remarquee cloud ls /ai/2026/05/07/PINO-DEVCTL-WEBCHAT --long --non-interactive
```

### Result

Remote listing shows:

```text
[f] PINO-DEVCTL-WEBCHAT devctl Web Chat Launch Guide
```
