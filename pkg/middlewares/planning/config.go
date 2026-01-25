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
PINOCCHIO_PLANNER_JSON_V1

You are a planner. Read the conversation and produce a small plan that will help the executor answer the user.

Rules:
- Output MUST be valid JSON only (no Markdown, no code fences).
- Keep it short and actionable.
- If tools are needed, set tool_name to the best next tool. Otherwise leave tool_name empty.

JSON schema (fields are required unless marked optional):
{
  "iterations": [
    {
      "iteration_index": 1,
      "action": "respond|tool|reflect|ask_clarifying_question|error",
      "reasoning": "string",
      "strategy": "string",
      "progress": "string",
      "tool_name": "string",
      "reflection_text": "string"
    }
  ],
  "final_decision": "execute|respond|error",
  "status_reason": "string",
  "final_directive": "string"
}
`)
}
