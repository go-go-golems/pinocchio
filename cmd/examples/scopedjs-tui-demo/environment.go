package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dop251/goja"
	gojengine "github.com/go-go-golems/go-go-goja/engine"
	ggjmodules "github.com/go-go-golems/go-go-goja/modules"
	_ "github.com/go-go-golems/go-go-goja/modules/fs"

	"github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
)

const demoToolName = "eval_project_ops"

var _ = systemPrompt

type demoScope struct {
	WorkspaceID   string
	WorkspaceRoot string
	Fixtures      demoFixtureSet
}

type demoMeta struct {
	WorkspaceID  string
	ProjectName  string
	FileCount    int
	TaskCount    int
	NoteCount    int
	FixtureLabel string
}

type webserverModule struct{}

func (m *webserverModule) Name() string { return "webserver" }

func (m *webserverModule) Doc() string {
	return "webserver exposes get(path, payload) and routes() for registering demo HTTP routes."
}

func (m *webserverModule) Loader(_ *goja.Runtime, moduleObj *goja.Object) {
	exports := moduleObj.Get("exports").(*goja.Object)
	routes := []map[string]any{}
	_ = exports.Set("get", func(path string, payload any) map[string]any {
		route := map[string]any{
			"method":  "GET",
			"path":    path,
			"payload": payload,
		}
		routes = append(routes, route)
		return route
	})
	_ = exports.Set("routes", func() []map[string]any {
		return routes
	})
}

type obsidianModule struct {
	workspaceRoot string
}

func (m *obsidianModule) Name() string { return "obsidian" }

func (m *obsidianModule) Doc() string {
	return "obsidian exposes createNote(title, body) and link(path) for demo note management."
}

func (m *obsidianModule) Loader(_ *goja.Runtime, moduleObj *goja.Object) {
	exports := moduleObj.Get("exports").(*goja.Object)
	_ = exports.Set("createNote", func(title string, body string) (map[string]any, error) {
		slug := slugify(title)
		relPath := filepath.ToSlash(filepath.Join("notes", slug+".md"))
		absPath := filepath.Join(m.workspaceRoot, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return nil, err
		}
		content := "# " + title + "\n\n" + body + "\n"
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			return nil, err
		}
		return map[string]any{
			"title": title,
			"path":  relPath,
		}, nil
	})
	_ = exports.Set("link", func(path string) string {
		return "[[" + strings.TrimSpace(path) + "]]"
	})
}

func demoEnvironmentSpec() scopedjs.EnvironmentSpec[demoScope, demoMeta] {
	return scopedjs.EnvironmentSpec[demoScope, demoMeta]{
		RuntimeLabel: "project-ops-demo",
		Tool: scopedjs.ToolDefinitionSpec{
			Name: demoToolName,
			Description: scopedjs.ToolDescription{
				Summary: "Execute JavaScript inside a scoped project workspace runtime with file, note, route, and task data helpers.",
				Notes: []string{
					"The runtime is scoped to one prepared workspace and should be used for workspace files, tasks, notes, and demo route setup.",
					"Prefer returning a concise structured object that summarizes created notes, written files, and registered routes.",
				},
				StarterSnippets: []string{
					`const rows = db.openTasks(); return rows;`,
					`const webserver = require("webserver"); webserver.get("/tasks", db.openTasks()); return webserver.routes();`,
				},
			},
			Tags:    []string{"pinocchio", "scopedjs", "demo", "javascript"},
			Version: "1.0.0",
		},
		DefaultEval: scopedjs.EvalOptions{
			Timeout:        5 * time.Second,
			MaxOutputChars: 16_000,
			CaptureConsole: true,
			StateMode:      scopedjs.StatePerCall,
		},
		Configure: configureDemoRuntime,
	}
}

func configureDemoRuntime(ctx context.Context, b *scopedjs.Builder, scope demoScope) (demoMeta, error) {
	fsModule := ggjmodules.GetModule("fs")
	if fsModule == nil {
		return demoMeta{}, fmt.Errorf("fs module is not registered")
	}
	if err := b.AddNativeModule(fsModule); err != nil {
		return demoMeta{}, err
	}
	if err := b.AddNativeModule(&webserverModule{}); err != nil {
		return demoMeta{}, err
	}
	if err := b.AddNativeModule(&obsidianModule{workspaceRoot: scope.WorkspaceRoot}); err != nil {
		return demoMeta{}, err
	}
	if err := b.AddGlobal("workspaceRoot", func(ctx *gojengine.RuntimeContext) error {
		return ctx.VM.Set("workspaceRoot", scope.WorkspaceRoot)
	}, scopedjs.GlobalDoc{
		Type:        "string",
		Description: "Writable root directory for the prepared demo workspace.",
	}); err != nil {
		return demoMeta{}, err
	}
	if err := b.AddGlobal("db", func(ctx *gojengine.RuntimeContext) error {
		return ctx.VM.Set("db", buildDBGlobal(scope.Fixtures))
	}, scopedjs.GlobalDoc{
		Type:        "object",
		Description: "Scoped project data helper with openTasks(), latestNotes(), and query(sql).",
	}); err != nil {
		return demoMeta{}, err
	}
	if err := b.AddBootstrapSource("helpers.js", `
function joinPath(a, b) {
  return a.replace(/\/$/, "") + "/" + b.replace(/^\//, "");
}
`); err != nil {
		return demoMeta{}, err
	}
	if err := b.AddHelper("joinPath", "joinPath(a, b)", "Join workspace-relative path segments."); err != nil {
		return demoMeta{}, err
	}

	fileCount := len(scope.Fixtures.Files) + len(scope.Fixtures.Notes)
	return demoMeta{
		WorkspaceID:  scope.WorkspaceID,
		ProjectName:  scope.Fixtures.ProjectName,
		FileCount:    fileCount,
		TaskCount:    len(scope.Fixtures.Tasks),
		NoteCount:    len(scope.Fixtures.Notes),
		FixtureLabel: scope.Fixtures.FixtureLabel,
	}, nil
}

func buildDBGlobal(fixtures demoFixtureSet) map[string]any {
	openTasks := make([]map[string]any, 0, len(fixtures.Tasks))
	for _, task := range fixtures.Tasks {
		openTasks = append(openTasks, map[string]any{
			"taskId":    task.TaskID,
			"title":     task.Title,
			"status":    task.Status,
			"priority":  task.Priority,
			"owner":     task.Owner,
			"updatedAt": task.UpdatedAt,
		})
	}

	notes := make([]map[string]any, 0, len(fixtures.Notes))
	for _, note := range fixtures.Notes {
		notes = append(notes, map[string]any{
			"title":     note.Title,
			"path":      note.Path,
			"updatedAt": note.UpdatedAt,
		})
	}
	sort.Slice(notes, func(i, j int) bool {
		return fmt.Sprint(notes[i]["updatedAt"]) > fmt.Sprint(notes[j]["updatedAt"])
	})

	return map[string]any{
		"openTasks": func() []map[string]any {
			return cloneRows(openTasks)
		},
		"latestNotes": func() []map[string]any {
			return cloneRows(notes)
		},
		"query": func(sql string) []map[string]any {
			rows := cloneRows(openTasks)
			for _, row := range rows {
				row["sql"] = sql
			}
			return rows
		},
	}
}

func buildDemoRegistry(ctx context.Context, workspaceID string) (*tools.InMemoryToolRegistry, demoMeta, func() error, error) {
	fixtures, err := demoFixturesForWorkspace(workspaceID)
	if err != nil {
		return nil, demoMeta{}, nil, err
	}
	materialized, err := materializeDemoWorkspace(fixtures)
	if err != nil {
		return nil, demoMeta{}, nil, err
	}
	scope := demoScope{
		WorkspaceID:   fixtures.WorkspaceID,
		WorkspaceRoot: materialized.Root,
		Fixtures:      fixtures,
	}
	spec := demoEnvironmentSpec()
	handle, err := scopedjs.BuildRuntime(ctx, spec, scope)
	if err != nil {
		_ = materialized.Cleanup()
		return nil, demoMeta{}, nil, err
	}

	registry := tools.NewInMemoryToolRegistry()
	if err := scopedjs.RegisterPrebuilt(registry, spec, handle, scopedjs.EvalOptions{}); err != nil {
		if handle.Cleanup != nil {
			_ = handle.Cleanup()
		}
		_ = materialized.Cleanup()
		return nil, demoMeta{}, nil, err
	}

	cleanup := func() error {
		var errs []string
		if handle.Cleanup != nil {
			if err := handle.Cleanup(); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if err := materialized.Cleanup(); err != nil {
			errs = append(errs, err.Error())
		}
		if len(errs) > 0 {
			return fmt.Errorf("%s", strings.Join(errs, "; "))
		}
		return nil
	}
	return registry, handle.Meta, cleanup, nil
}

func systemPrompt(meta demoMeta) string {
	return strings.TrimSpace(fmt.Sprintf(`
You are a project operations assistant.

The available JavaScript tool is scoped to workspace %s for project %s. Use it when the user asks about workspace files, open tasks, notes, generated summaries, or demo routes.

The runtime already exposes fs, db, require("obsidian"), require("webserver"), workspaceRoot, and helper functions such as joinPath(...).

Prefer one coherent tool call that returns a concise structured result. After reading the tool output, answer in plain English and mention concrete file paths, note paths, or route paths when useful.
`, meta.WorkspaceID, meta.ProjectName))
}

func executeDemoEval(ctx context.Context, reg *tools.InMemoryToolRegistry, in scopedjs.EvalInput) (scopedjs.EvalOutput, error) {
	def, err := reg.GetTool(demoToolName)
	if err != nil {
		return scopedjs.EvalOutput{}, err
	}
	args, err := json.Marshal(in)
	if err != nil {
		return scopedjs.EvalOutput{}, err
	}
	result, err := def.Function.ExecuteWithContext(ctx, args)
	if err != nil {
		return scopedjs.EvalOutput{}, err
	}
	out, ok := result.(scopedjs.EvalOutput)
	if !ok {
		return scopedjs.EvalOutput{}, fmt.Errorf("unexpected result type %T", result)
	}
	return out, nil
}

func cloneRows(rows []map[string]any) []map[string]any {
	ret := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		clone := make(map[string]any, len(row))
		for k, v := range row {
			clone[k] = v
		}
		ret = append(ret, clone)
	}
	return ret
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "note"
	}
	return s
}
