Based on the stream data shown, I'll recommend the key metadata fields that would be most informative while keeping the output compact. Here's my analysis:

Most important fields to show:

1. For the initial `start` event:
- `type` (start/partial/final)
- `step_metadata.type` (e.g. claude-chat)
- `step_metadata.metadata.claude_model`
- Settings that affect output:
  - ai-temperature
  - ai-max-response-tokens

2. For `partial` events:
- `type`
- `content` (the actual streamed text)

3. For the `final` event:
- `type`
- `content`
- Usage statistics:
  - input_tokens
  - output_tokens
- `claude_stop_reason`

Here's a suggested compact format:

```yaml:pinocchio/ttmp/2024-12-30/02-test-stream.yaml
# Start event
start:
  type: claude-chat
  model: claude-3-5-haiku-20241022
  temp: 0.2
  max_tokens: 8192

# Partial events
partial: "As"
partial: " an AI"
partial: ","
# ... more partials ...

# Final event
final:
  content: "As an AI, I don't actually have..."
  tokens: {in: 22, out: 84}
  stop_reason: end_turn
```

This format provides:
1. Initial context about the model and key settings
2. Streaming progress via partial content
3. Final summary with token usage and completion status
