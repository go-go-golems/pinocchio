---
Title: Migrating Legacy Pinocchio Config to Unified Profile Documents
Slug: config-migration-guide
Short: Step-by-step migration guide from legacy Pinocchio config keys to the unified app/profile/profiles document model.
Topics:
- pinocchio
- config
- migration
- profiles
- tutorial
Commands:
- pinocchio
- js
Flags:
- config-file
- profile
- profile-registries
- print-inference-settings
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

## What This Guide Covers

This guide explains how to rewrite older Pinocchio config files to the current unified document model.

Use it when your config still contains legacy keys such as:

- `repositories` at the top level
- `profile-settings.*`
- `ai-chat`
- `openai-chat`
- `claude-chat`
- `.pinocchio-profile.yml`

The new model is intentionally stricter. Pinocchio now expects one unified config document with three semantic blocks:

- `app`
- `profile`
- `profiles`

That means old top-level runtime sections are not read anymore. If you leave them in place, Pinocchio will fail loudly instead of silently guessing.

## The New Mental Model

The old model mixed several concerns together in one flat app config file:

- prompt repository locations
- active/default profile selection
- imported profile registries
- default provider/model settings
- provider-specific settings

The new model separates those concerns explicitly:

- `app` contains Pinocchio-owned application settings
- `profile` contains profile selection plus imported registry sources
- `profiles` contains inline engine-profile definitions stored directly in the config document

A minimal unified document looks like this:

```yaml
app:
  repositories:
    - ~/code/prompts

profile:
  active: default

profiles:
  default:
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini
```

## Old-to-New Mapping

Use this table as the quick translation reference.

| Legacy shape | New shape | Notes |
|---|---|---|
| top-level `repositories:` | `app.repositories:` | Pinocchio-owned application config |
| `profile-settings.profile` | `profile.active` | Selected/default profile |
| `profile-settings.profile-registries` | `profile.registries` | Imported engine-only registries |
| `.pinocchio-profile.yml` | `.pinocchio.yml` | Local project override filename |
| top-level `ai-chat`, `openai-chat`, `claude-chat`, similar runtime sections | `profiles.<slug>.inference_settings` or external `profiles.yaml` | There is no new top-level runtime block |

The most important migration rule is this:

**There is no direct top-level replacement for old runtime sections.**

If you previously kept model/provider defaults in top-level `ai-chat` or provider sections, move them into:

- an inline profile under `profiles.<slug>.inference_settings`, or
- an external engine-only `profiles.yaml` referenced from `profile.registries`

## Step 1: Rename Local Project Files

If you have a repository-local or working-directory-local override file named:

```text
.pinocchio-profile.yml
```

rename it to:

```text
.pinocchio.yml
```

Pinocchio now rejects the old filename.

The supported local discovery order is:

1. git-root `.pinocchio.yml`
2. current-working-directory `.pinocchio.yml`
3. explicit `--config-file`

## Step 2: Move Top-Level Repositories into `app.repositories`

Older config often used:

```yaml
repositories:
  - ~/code/prompts
  - ~/.pinocchio/repository
```

Rewrite that to:

```yaml
app:
  repositories:
    - ~/code/prompts
    - ~/.pinocchio/repository
```

This is the easiest part of the migration.

## Step 3: Move `profile-settings.*` into `profile.*`

Older config often looked like this:

```yaml
profile-settings:
  profile: assistant
  profile-registries:
    - ~/.config/pinocchio/profiles.yaml
```

Rewrite it to:

```yaml
profile:
  active: assistant
  registries:
    - ~/.config/pinocchio/profiles.yaml
```

Use:

- `profile.active` for the selected/default profile slug
- `profile.registries` for imported engine-only registry sources

## Step 4: Choose Inline Profiles or Imported Registries

At this point, you need to decide where the actual engine settings live.

### Option A: Keep everything in one unified config document

This is the simplest migration when you previously had one personal config file with one default model.

Use inline profiles:

```yaml
app:
  repositories:
    - ~/.pinocchio/repository

profile:
  active: assistant

profiles:
  assistant:
    display_name: Assistant
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini
```

This is the best target when:

- you only need a few local profiles
- you want one file to control both app config and profile selection
- you do not need a shared team profile catalog

### Option B: Keep engine profiles in `profiles.yaml`

This is the best target when you already have an external engine-profile registry file.

Unified app config:

```yaml
app:
  repositories:
    - ~/.pinocchio/repository

profile:
  active: assistant
  registries:
    - ~/.config/pinocchio/profiles.yaml
```

External engine-only registry:

```yaml
slug: workspace
profiles:
  assistant:
    slug: assistant
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini
```

This is the best target when:

- you already share `profiles.yaml` across multiple commands or machines
- you want to keep the app config document small
- you want inline profiles only for local overrides, not for the main catalog

## Step 5: Migrate Old Runtime Sections into `profiles.<slug>.inference_settings`

This is the part that usually causes confusion.

Older config often kept engine settings in top-level runtime sections such as:

```yaml
ai-chat:
  ai-api-type: openai
  ai-engine: gpt-5-mini
openai-chat:
  openai-api-key: ...
claude-chat:
  claude-api-key: ...
```

Do not try to keep these at the top level. The unified document does not support that shape.

Instead, decide what role those settings were playing.

### If they described your default model/provider

Create an inline profile and make it active:

```yaml
profile:
  active: default

profiles:
  default:
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini
```

### If they belonged in a shared profile catalog

Move them into the engine-only `profiles.yaml` registry format and reference that file from `profile.registries`.

### If they were shared credentials or cross-profile defaults

Prefer environment variables rather than copying secrets into every profile.

For example:

```bash
export OPENAI_API_KEY=...
export ANTHROPIC_API_KEY=...
```

This is usually cleaner than repeating credentials inside multiple inline profiles.

If you truly need profile-specific credentials, keep them under that profile's `inference_settings`. But avoid committing secrets to repository-local `.pinocchio.yml` files.

## Concrete Before-and-After Examples

## Legacy single-file config

```yaml
repositories:
  - ~/.pinocchio/repository

profile-settings:
  profile: assistant
  profile-registries:
    - ~/.config/pinocchio/profiles.yaml

ai-chat:
  ai-api-type: openai
  ai-engine: gpt-5-mini

openai-chat:
  openai-api-key: <secret>
```

## New inline unified config

```yaml
app:
  repositories:
    - ~/.pinocchio/repository

profile:
  active: assistant

profiles:
  assistant:
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini
```

Then keep credentials in the environment:

```bash
export OPENAI_API_KEY=<secret>
```

## New unified config plus imported registry

`~/.config/pinocchio/config.yaml`:

```yaml
app:
  repositories:
    - ~/.pinocchio/repository

profile:
  active: assistant
  registries:
    - ~/.config/pinocchio/profiles.yaml
```

`~/.config/pinocchio/profiles.yaml`:

```yaml
slug: workspace
profiles:
  assistant:
    slug: assistant
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini
```

## Validation Checklist

After rewriting the config, validate it with an ordinary Pinocchio command.

Recommended checks:

1. Confirm the command still sees your prompt repositories.
2. Confirm the selected profile resolves correctly.
3. Confirm `--print-inference-settings` shows the expected final model/provider.
4. Confirm old filenames and old top-level keys are gone.

Example commands:

```bash
pinocchio examples test --print-inference-settings

pinocchio js --script examples/js/runner-profile-smoke.js --print-inference-settings
```

If you rely on a specific config file during the migration, use:

```bash
pinocchio --config-file ~/.config/pinocchio/config.yaml examples test --print-inference-settings
```

## Common Migration Mistakes

### Keeping `profile-settings` at the top level

This now fails. Rewrite it to `profile.active` and `profile.registries`.

### Renaming the file but not the keys

Changing `.pinocchio-profile.yml` to `.pinocchio.yml` is necessary, but not sufficient. The document contents must also be rewritten.

### Keeping `ai-chat` as a top-level block

There is no supported new top-level runtime block. Put model/provider settings under an inline profile or an imported engine-only registry.

### Treating `profiles.yaml` as an app-runtime document

`profiles.yaml` is now engine-only. Keep prompts, tool choices, middleware choices, and repository settings out of it.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `field profile-settings not found in type configdoc.Document` | The config file still uses the old `profile-settings` block | Rewrite it to `profile.active` and `profile.registries` |
| `field ai-chat not found in type configdoc.Document` | The config file still uses old top-level runtime sections | Move those settings into `profiles.<slug>.inference_settings` or `profiles.yaml` |
| `legacy local config filename ".pinocchio-profile.yml" is no longer supported` | The local override file still uses the old filename | Rename it to `.pinocchio.yml` |
| A profile resolves but credentials are missing | Shared provider credentials were removed from old top-level provider blocks during migration | Set them via environment variables or add them intentionally to the target profile |
| Prompt repositories disappeared | `repositories` stayed at the top level instead of moving under `app` | Rewrite to `app.repositories` |

## See Also

- [Pinocchio Profile Resolution and Runtime Switching](../topics/pinocchio-profile-resolution-and-runtime-switching.md)
- [Pinocchio CLI Verb Migration Guide](07-migrating-cli-verbs-to-glazed-profile-bootstrap.md)
- [Webchat Engine Profile Guide](../topics/webchat-profile-registry.md)
- [JS Runner Scripts](../../../cmd/pinocchio/doc/general/05-js-runner-scripts.md)
