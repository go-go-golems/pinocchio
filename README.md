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

Configure pinocchio through layered config plus an engine-profile registry stack.
Use `~/.pinocchio/config.yaml` for app config and optional provider defaults, and use
engine profiles to select the active model/provider settings.

```yaml
repositories:
  - /Users/manuel/code/pinocchio
  - /Users/manuel/.pinocchio/repository
profile-settings:
  profile-registries: ~/.config/pinocchio/profiles.yaml
openai-chat:
  openai-api-key: XXXX
```

Do not use the legacy flat `openai-api-key: ...` top-level shape for new config files.

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
3. `profile-settings.profile-registries` in the selected config file
4. `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml` when the file exists

Profile-selection precedence:

1. `--profile`
2. `PINOCCHIO_PROFILE`
3. `profile-settings.profile` in the selected config file

## Repository loading from layered config

Pinocchio command repositories are **not** part of the shared Geppetto section model.
They stay as a Pinocchio-local top-level config key:

- `repositories`

That means two related but different config passes happen at startup:

1. shared Geppetto/Pinocchio bootstrap loads section-shaped config such as `profile-settings`, `ai-chat`, and `ai-client`
2. the root CLI then separately reads `repositories` from the resolved Pinocchio config files in `cmd/pinocchio/main.go`

Current behavior in `loadRepositoriesFromConfig()` is:

1. resolve the config-file stack through the same Pinocchio config plan used by bootstrap
2. read the top-level `repositories` list from **every** resolved config file, not just the highest-precedence one
3. append repository entries in resolved-config order
4. de-duplicate exact repeated repository strings
5. append the default local prompt directory `$HOME/.pinocchio/prompts`
6. mount only directories that actually exist on disk

This split is intentional:

- shared bootstrap should not try to interpret Pinocchio-specific repository metadata
- Pinocchio still needs repository loading to follow the same config-file discovery rules as profile/config resolution

If you are debugging command discovery, inspect both:

- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `pinocchio/cmd/pinocchio/main.go`
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

`pinocchio js` also accepts `--config-file`, so the script can inherit `profile-settings.profile-registries` and `profile-settings.profile` from the same config file used by the other Pinocchio commands.

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
      api:
        api_keys:
          openai-api-key: your-api-key
      chat:
        api_type: openai
        engine: gpt-4o-mini
```

The old outer wrapper key `inference_settings.api_keys` is no longer supported. Use `inference_settings.api` instead.

Use [examples/js/profiles/basic.yaml](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/profiles/basic.yaml) as the smallest concrete reference.

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

For aliases stored in nested directories, `aliasFor` can also point to an explicit full command path:

```yaml
name: concise-doc
aliasFor: [code, go]
```

This is useful when the alias file lives under a subdirectory like `prompts/code/go/` but the target command is the full path `code go`.

## Contributing

This is GO GO GOLEMS playground, and GO GO GOLEMS don't accept contributions.
The structure of the project will significantly change as we go forward, but
the core concept of a declarative prompting structure will stay the same,
and as such, you should be reasonably safe writing YAMLs to be used with pinocchio.
