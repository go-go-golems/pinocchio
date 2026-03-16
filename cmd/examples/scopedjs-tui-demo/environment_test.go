package main

import (
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
)

func TestBuildDemoRegistryRegistersToolAndMeta(t *testing.T) {
	t.Parallel()

	registry, meta, cleanup, err := buildDemoRegistry(context.Background(), "apollo")
	if cleanup != nil {
		defer func() { _ = cleanup() }()
	}
	if err != nil {
		t.Fatalf("buildDemoRegistry returned error: %v", err)
	}
	if registry == nil {
		t.Fatalf("expected registry")
	}
	if !registry.HasTool(demoToolName) {
		t.Fatalf("expected tool %q to be registered", demoToolName)
	}
	if meta.ProjectName != "Apollo Dashboard Refresh" {
		t.Fatalf("unexpected project name: %q", meta.ProjectName)
	}
	if meta.TaskCount != 3 {
		t.Fatalf("unexpected task count: %d", meta.TaskCount)
	}
	if meta.NoteCount != 2 {
		t.Fatalf("unexpected note count: %d", meta.NoteCount)
	}
}

func TestExecuteDemoEvalComposesRuntimeCapabilities(t *testing.T) {
	t.Parallel()

	registry, _, cleanup, err := buildDemoRegistry(context.Background(), "apollo")
	if cleanup != nil {
		defer func() { _ = cleanup() }()
	}
	if err != nil {
		t.Fatalf("buildDemoRegistry returned error: %v", err)
	}

	out, err := executeDemoEval(context.Background(), registry, scopedjs.EvalInput{
		Code: `
const fs = require("fs");
const obsidian = require("obsidian");
const webserver = require("webserver");

const rows = db.openTasks();
const note = obsidian.createNote("Task Digest", rows.map((row) => "- " + row.title).join("\n"));
const summaryPath = joinPath(workspaceRoot, "artifacts/summary.json");
fs.writeFileSync(summaryPath, JSON.stringify(rows));
webserver.get("/tasks", { count: rows.length, notePath: note.path });

return {
  summaryPath,
  note,
  routes: webserver.routes(),
  rows,
};
`,
	})
	if err != nil {
		t.Fatalf("executeDemoEval returned error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("expected no eval error, got %q", out.Error)
	}
	result, ok := out.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", out.Result)
	}
	if !strings.Contains(result["summaryPath"].(string), "artifacts/summary.json") {
		t.Fatalf("unexpected summary path: %v", result["summaryPath"])
	}
	note, ok := result["note"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected note result: %T", result["note"])
	}
	if note["path"] != "notes/task-digest.md" {
		t.Fatalf("unexpected note path: %v", note["path"])
	}
	routes, ok := result["routes"].([]map[string]any)
	if !ok || len(routes) != 1 {
		t.Fatalf("unexpected routes payload: %#v", result["routes"])
	}
}

func TestExecuteDemoEvalAllowsCallbackStyleRoutes(t *testing.T) {
	t.Parallel()

	registry, _, cleanup, err := buildDemoRegistry(context.Background(), "apollo")
	if cleanup != nil {
		defer func() { _ = cleanup() }()
	}
	if err != nil {
		t.Fatalf("buildDemoRegistry returned error: %v", err)
	}

	out, err := executeDemoEval(context.Background(), registry, scopedjs.EvalInput{
		Code: `
const webserver = require("webserver");
const tasks = db.openTasks();
webserver.get("/tasks", () => tasks);
return webserver.routes();
`,
	})
	if err != nil {
		t.Fatalf("executeDemoEval returned error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("expected no eval error, got %q", out.Error)
	}
	routes, ok := out.Result.([]map[string]any)
	if !ok || len(routes) != 1 {
		t.Fatalf("unexpected routes result: %#v", out.Result)
	}
	if routes[0]["path"] != "/tasks" {
		t.Fatalf("unexpected route path: %v", routes[0]["path"])
	}
	if routes[0]["payload"] != "[function]" {
		t.Fatalf("unexpected route payload: %#v", routes[0]["payload"])
	}
}

func TestExecuteDemoEvalReportsJavaScriptErrors(t *testing.T) {
	t.Parallel()

	registry, _, cleanup, err := buildDemoRegistry(context.Background(), "apollo")
	if cleanup != nil {
		defer func() { _ = cleanup() }()
	}
	if err != nil {
		t.Fatalf("buildDemoRegistry returned error: %v", err)
	}

	out, err := executeDemoEval(context.Background(), registry, scopedjs.EvalInput{
		Code: `throw new Error("demo failure");`,
	})
	if err != nil {
		t.Fatalf("executeDemoEval returned error: %v", err)
	}
	if strings.TrimSpace(out.Error) == "" {
		t.Fatalf("expected non-empty eval error, got %#v", out.Error)
	}
}

func TestDemoFixturesForUnknownWorkspace(t *testing.T) {
	t.Parallel()

	_, err := demoFixturesForWorkspace("missing")
	if err == nil {
		t.Fatalf("expected error for unknown workspace")
	}
}
