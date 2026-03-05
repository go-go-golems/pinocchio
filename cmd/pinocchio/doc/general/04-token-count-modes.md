---
Title: Token Count Modes
Slug: token-count-modes
Short: Choosing between local estimates and provider-native token counting
Topics:
- tokens
- token-count
- openai
- claude
- pinocchio
Commands:
- pinocchio
- tokens
Flags:
- count-mode
- model
- codec
- ai-api-type
- profile
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Overview

`pinocchio tokens count` now supports three counting modes through `--count-mode`:

- `estimate`: local tokenizer estimate using the existing tiktoken-based path
- `api`: provider-native token counting through Geppetto
- `auto`: try provider-native counting first and fall back to a local estimate

## Basic Examples

Local estimate:

```bash
pinocchio tokens count --count-mode estimate --model gpt-4o-mini prompt.txt
```

OpenAI Responses API count:

```bash
pinocchio tokens count \
  --count-mode api \
  --model gpt-4o-mini \
  --ai-api-type openai-responses \
  --openai-api-key "$OPENAI_API_KEY" \
  prompt.txt
```

Anthropic count with profile or explicit flags:

```bash
pinocchio tokens count \
  --count-mode api \
  --model claude-sonnet-4-20250514 \
  --ai-api-type claude \
  --claude-api-key "$ANTHROPIC_API_KEY" \
  prompt.txt
```

Automatic fallback:

```bash
pinocchio tokens count --count-mode auto --model gpt-4o-mini prompt.txt
```

## How To Choose

- Use `estimate` when you want a fast local answer and do not need provider-exact counts.
- Use `api` when the exact provider accounting matters.
- Use `auto` when you prefer provider-exact counts but still want the command to work without credentials or when provider-native counting is unavailable.

## Output Shape

The command prints the requested mode and the actual count source so fallback behavior is explicit.

- Estimate output includes the tokenizer codec used.
- API output includes the provider and endpoint used.
- Auto fallback output includes the provider error that triggered the local estimate.
