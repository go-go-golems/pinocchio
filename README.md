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

Configure pinocchio through layered config plus a profile-registry stack.
Use `~/.pinocchio/config.yaml` for app config and optional provider defaults, and use
profile registries to select the active model/provider runtime.

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

## Profile registry loading

Pinocchio now resolves profiles from a profile-registry source stack.

Selection knobs:

- `--profile-registries` (comma-separated YAML/SQLite sources)
- `PINOCCHIO_PROFILE_REGISTRIES`
- `profile-settings.profile-registries` in config YAML

When none of the above are set, pinocchio automatically uses:

- `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml` (if the file exists)

Profile selection is still done with:

- `--profile`
- `PINOCCHIO_PROFILE`

Example:

```bash
PINOCCHIO_PROFILE=gpt-5 pinocchio examples test
```

This works as long as `gpt-5` exists in the default runtime file (`~/.config/pinocchio/profiles.yaml`) or one of the configured profile-registry sources.

Runtime YAML format is single-registry only:

```yaml
slug: default
profiles:
  gpt-5:
    slug: gpt-5
    runtime:
      system_prompt: You are the GPT-5 assistant profile.
      tools:
        - calculator
```

Do not use `registries:` or `default_profile_slug` in runtime YAML sources.
Keep provider credentials and other base defaults in layered app config, and use profiles for prompt/tool/middleware metadata only.

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

- Pinocchio owns config/env/default resolution for hidden base `StepSettings`
- Pinocchio owns profile-registry discovery
- Geppetto still owns the generic JS inference and runner API

That means a script can resolve runtime from profile registries and build an engine from the same layered Pinocchio config the CLI already uses.

Example:

```javascript
const gp = require("geppetto");
const pinocchio = require("pinocchio");

const engine = pinocchio.engines.fromDefaults({
  model: "gpt-4o-mini",
  apiType: "openai",
});

const runtime = gp.runner.resolveRuntime({
  profile: { profileSlug: "assistant" },
});

const out = gp.runner.run({
  engine,
  runtime,
  prompt: "Summarize the repo in one line.",
});

console.log(out.blocks[0].payload.text);
```

There is a runnable local example in:

- [examples/js/runner-profile-demo.js](./examples/js/runner-profile-demo.js)

Run it with:

```bash
pinocchio js \
  --script examples/js/runner-profile-demo.js \
  --profile-registries examples/js/profiles/basic.yaml
```

## Migrating old profiles.yaml

If your old file used the legacy map format, automatic migration is no longer available. Rebuild the registry manually with `runtime.system_prompt`, `runtime.tools`, and `runtime.middlewares`, and move engine/provider settings into app config.

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
