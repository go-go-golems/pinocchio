package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
)

func TestBuildEvalToolCallMarkdownRendersJSAndInput(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(scopedjs.EvalInput{
		Code:  "const rows = db.openTasks(); return rows;",
		Input: map[string]any{"limit": 3},
	})
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}

	md := buildEvalToolCallMarkdown(demoToolName, demoToolName, string(raw))
	if !strings.Contains(md, "```js") {
		t.Fatalf("expected js fence, got %q", md)
	}
	if !strings.Contains(md, "db.openTasks()") {
		t.Fatalf("expected code body, got %q", md)
	}
	if !strings.Contains(md, "limit: 3") {
		t.Fatalf("expected yaml input, got %q", md)
	}
}

func TestFormatEvalResultMarkdownSummarizesStructuredResult(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(scopedjs.EvalOutput{
		Result: map[string]any{
			"summaryPath": "artifacts/summary.json",
			"note": map[string]any{
				"title": "Task Digest",
				"path":  "notes/task-digest.md",
			},
			"routes": []map[string]any{
				{"method": "GET", "path": "/tasks"},
			},
			"rows": []map[string]any{
				{"taskId": "APL-101", "title": "Unify dashboard card spacing"},
			},
		},
		Console: []scopedjs.ConsoleLine{
			{Level: "log", Text: "created note"},
		},
		DurationMs: 12,
	})
	if err != nil {
		t.Fatalf("marshal output: %v", err)
	}

	md := formatEvalResultMarkdown(string(raw))
	for _, expected := range []string{
		"summaryPath",
		"notes/task-digest.md",
		"GET /tasks",
		"created note",
		"duration: `12ms`",
	} {
		if !strings.Contains(md, expected) {
			t.Fatalf("expected %q in markdown:\n%s", expected, md)
		}
	}
}

func TestFormatEvalResultMarkdownShowsErrors(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(scopedjs.EvalOutput{
		Error: "Promise rejected: boom",
	})
	if err != nil {
		t.Fatalf("marshal output: %v", err)
	}

	md := formatEvalResultMarkdown(string(raw))
	if !strings.Contains(md, "**Error**") {
		t.Fatalf("expected error heading, got %q", md)
	}
	if !strings.Contains(md, "boom") {
		t.Fatalf("expected error text, got %q", md)
	}
}
