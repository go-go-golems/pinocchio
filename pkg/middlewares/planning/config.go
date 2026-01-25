package planning

import "strings"

// Config controls the planning lifecycle wrapper.
type Config struct {
	Enabled       bool
	MaxIterations int
	Prompt        string
}

// DefaultConfig returns the default planning configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		MaxIterations: 3,
		Prompt:        defaultPlannerPrompt(),
	}
}

// Sanitized returns a validated copy with defaults filled in.
func (c Config) Sanitized() Config {
	out := c
	if out.MaxIterations <= 0 {
		out.MaxIterations = DefaultConfig().MaxIterations
	}
	out.Prompt = strings.TrimSpace(out.Prompt)
	if out.Prompt == "" {
		out.Prompt = DefaultConfig().Prompt
	}
	return out
}

func defaultPlannerPrompt() string {
	// This is a pragmatic, parser-friendly prompt tuned for:
	// - producing incremental "planning.iteration" events
	// - generating a final directive suitable for executor/toolloop
	return strings.TrimSpace(`
PINOCCHIO_PLANNER_ITER_V1

You are a planner. Read the conversation and produce ONE planning iteration at a time.

Rules:
- Output MUST be valid JSON only (no Markdown, no code fences).
- Keep it short and actionable.
- You will be called repeatedly. Use the provided STATE_JSON to decide what iteration to emit next.
- If tools are needed, set tool_name to the best next tool. Otherwise leave tool_name empty.
- Always fill final_decision and final_directive; they may be updated on later iterations.

JSON schema (fields are required unless marked optional):
{
  "iteration": {
    "iteration_index": 1,
    "action": "respond|tool|reflect|ask_clarifying_question|error",
    "reasoning": "string",
    "strategy": "string",
    "progress": "string",
    "tool_name": "string",
    "reflection_text": "string"
  },
  "continue": true,
  "final_decision": "execute|respond|error",
  "status_reason": "string",
  "final_directive": "string"
}

STATE_JSON will be appended after this prompt and is authoritative for iteration_index and max_iterations.
`)
}
