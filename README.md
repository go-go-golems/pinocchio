# Pinocchio - CLI LLM tool


`pinocchio` is a tool that can be used to interact
with different prompting applications interactively or from the command line.

### Installation

To install the `pinocchio` command line tool with homebrew, run:

```bash
brew tap go-go-golems/go-go-go
brew install go-go-golems/go-go-go/pinocchio
```

To install the `pinocchio` command using apt-get, run:

```bash
echo "deb [trusted=yes] https://apt.fury.io/go-go-golems/ /" >> /etc/apt/sources.list.d/fury.list
apt-get update
apt-get install pinocchio
```

To install using `yum`, run:

```bash
echo "
[fury]
name=Gemfury Private Repo
baseurl=https://yum.fury.io/go-go-golems/
enabled=1
gpgcheck=0
" >> /etc/yum.repos.d/fury.repo
yum install pinocchio
```

To install using `go get`, run:

```bash
go get -u github.com/go-go-golems/pinocchio/cmd/pinocchio
```

Finally, install by downloading the binaries straight from [github](https://github.com/go-go-golems/geppetto/releases).

## Usage

Configure Pinocchio through layered unified config documents plus an optional engine-profile registry stack.

The unified config shape is:

- `app`: application-level settings such as prompt repositories
- `profile`: the selected/default profile plus optional imported registry sources
- `profiles`: inline profile definitions stored directly in the same config document

Pinocchio reads the standard global config files (`/etc/pinocchio/config.yaml`, `$HOME/.pinocchio/config.yaml`, `${XDG_CONFIG_HOME}/pinocchio/config.yaml`) plus local project config from `.pinocchio.yml` and optional uncommitted override layers from `.pinocchio.override.yml` at the git root and current working directory.

Example:

```yaml
app:
  repositories:
    - /Users/manuel/code/pinocchio
    - /Users/manuel/.pinocchio/repository

profile:
  active: default
  registries:
    - ~/.config/pinocchio/profiles.yaml

profiles:
  default:
    display_name: Default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini
```

Do not use legacy config shapes such as `profile-settings`, `ai-chat`, `openai-chat`, or the old local filename `.pinocchio-profile.yml`.

For a step-by-step rewrite from the old format, see [Migrating Legacy Pinocchio Config to Unified Profile Documents](./pkg/doc/tutorials/08-migrating-legacy-pinocchio-config-to-unified-profile-documents.md).

You can then start using `pinocchio`:

```bash
❯ pinocchio examples test --print-prompt
Pretend you are a scientist. What is the age of you?

❯ pinocchio examples test               

As a scientist, I do not have an age.

❯ pinocchio examples test --pretend "100 year old explorer" --print-prompt
Pretend you are a 100 year old explorer. What is the age of you?

❯ pinocchio examples test --pretend "100 year old explorer"               

I am 100 years old.
```

Pinocchio comes with a selection of [demo prompts](https://github.com/go-go-golems/geppetto/tree/main/cmd/pinocchio/prompts/examples)
as an inspiration.

## Engine profile loading

Pinocchio resolves engine profiles from a registry source stack.

Registry-source precedence:

1. `--profile-registries` (comma-separated YAML/SQLite sources)
2. `PINOCCHIO_PROFILE_REGISTRIES`
3. `profile.registries` from the merged unified config document
4. `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml` when the file exists

Profile-selection precedence:

1. `--profile`
2. `PINOCCHIO_PROFILE`
3. `profile.active` from the merged unified config document
4. the registry default profile (`default_profile_slug` or slug `default`)

Example:

```bash
PINOCCHIO_PROFILE=gpt-5-mini pinocchio examples test
```

This works as long as `gpt-5-mini` exists in the default engine-profile file (`~/.config/pinocchio/profiles.yaml`) or one of the configured registry sources.

Engine-profile YAML format is single-registry only:

```yaml
slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-4o-mini
  gpt-5-mini:
    slug: gpt-5-mini
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5-mini
```

Keep prompts, middlewares, and tool selection out of this file. Those are application-level concerns now.

## Running JavaScript scripts

Pinocchio can now run JavaScript directly:

```bash
pinocchio js --script script.js
```

This command bootstraps a JS runtime with two important modules:

- `require("geppetto")`
  - the Geppetto JS API, including `gp.engines.*`, `gp.profiles.*`, and `gp.runner.*`
- `require("pinocchio")`
  - Pinocchio-specific helpers, starting with `pinocchio.engines.fromDefaults()`

The intended model is:

- Pinocchio owns config/env/default resolution for hidden base `InferenceSettings`
- Pinocchio owns engine-profile registry discovery
- Geppetto owns engine-profile resolution plus the generic JS inference and runner API

That means a script can resolve an engine profile from the same registry/config path the CLI already uses and then build an engine directly from that resolved profile.

`pinocchio js` also accepts `--config-file`, so the script can inherit `profile.registries` and `profile.active` from the same unified config document used by the other Pinocchio commands.

Example:

```javascript
const gp = require("geppetto");
const resolved = gp.profiles.resolve({});
console.log(JSON.stringify({
  profileSlug: resolved.profileSlug,
  model: resolved.inferenceSettings?.chat?.engine,
}, null, 2));

const engine = gp.engines.fromResolvedProfile(resolved);

const out = gp.runner.run({
  engine,
  prompt: "Summarize the repo in one line.",
});

console.log(out.blocks[0].payload.text);
```

There is a runnable real-inference example in:

- [examples/js/runner-profile-demo.js](./examples/js/runner-profile-demo.js)

It builds the engine directly from the resolved engine profile, so the selected profile controls the model/provider settings used for the actual inference.

Run it with:

```bash
pinocchio js \
  --script examples/js/runner-profile-demo.js \
  --profile-registries examples/js/profiles/basic.yaml
```

Or select a specific profile from that registry:

```bash
pinocchio js \
  examples/js/runner-profile-demo.js \
  --profile assistant \
  --profile-registries examples/js/profiles/basic.yaml
```

If you want a deterministic local smoke run instead of a live model call, use:

```bash
pinocchio js \
  --script examples/js/runner-profile-smoke.js \
  --profile-registries examples/js/profiles/basic.yaml
```

## profiles.yaml format

`pinocchio` now expects engine-only profiles in `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml`.

Old mixed-runtime profile files should be rewritten directly to the engine-only `inference_settings` shape. Prompt, middleware, and tool policy no longer belong in this file.

The target shape looks like this:

```yaml
slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-4o-mini
```

Use [examples/js/profiles/basic.yaml](./examples/js/profiles/basic.yaml) as the smallest concrete reference.

## Creating your own prompt

Creating your own prompt is easy. Create a yaml file in one of the configure repositories.
The directory layout will be mapped to the command verb hierarchy. For example,
the file `~/.pinocchio/repository/prompts/examples/test.yaml` will be available as the command
`pinocchio examples test`.

A prompt description is a yaml file with the following structure, as shown for a prompt
that can be used to rewrite text in a certain style. After a short description, the
flags and arguments configure how what variables will be used to interpolate the prompt at
the bottom.

```yaml
name: command-name
short: Rewrite text in a certain style
flags:
  - name: author
    type: stringList
    help: Inspired by authors
    default:
      - L. Ron Hubbard
      - Isaac Asimov
      - Richard Bandler
      - Robert Anton Wilson
  - name: adjective
    type: stringList
    help: Style adjectives
    default:
      - esoteric
      - retro
      - technical
      - seventies hip
      - science fiction
  - name: style
    type: string
    help: Style
    default: in a style reminiscent of seventies and eighties computer manuals
  - name: instructions
    type: string
    help: Additional instructions
arguments:
  - name: body
    type: stringFromFile
    help: Paragraph to rewrite
    required: true
prompt: |
  Rewrite the following paragraph, 
  {{ if .style }}in the style of {{ .style }},{{ end }}
  {{ if .adjective }}so that it sounds {{ .adjective | join ", " }}, {{ end }}
  {{ if .author }}in the style of {{ .author | join ", " }}. {{ end }}
  Don't mention any authors names.

  ---
  {{ .body -}}
  ---

  {{ if .instructions }} {{ .instructions }} {{ end }}

  ---
```

## Creating aliases

In addition to prompts, you can define aliases, which are just shortcuts to other commands, with certain flags
prefilled. The resulting yaml file can be placed alongside other commands in one of the configured repositories.

```shell
❯ pinocchio examples test --pretend "100 year old explorer" --create-alias old-explorer \
   | tee ~/.pinochio/repository/prompts/examples/old-explorer.yaml
name: old-explorer
aliasFor: test
flags:
    pretend: 100 year old explorer

❯ pinocchio examples old-explorer
I am 100 years old.
```

## Contributing

This is GO GO GOLEMS playground, and GO GO GOLEMS don't accept contributions.
The structure of the project will significantly change as we go forward, but
the core concept of a declarative prompting structure will stay the same,
and as such, you should be reasonably safe writing YAMLs to be used with pinocchio.
