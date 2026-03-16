package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type demoTask struct {
	TaskID    string
	Title     string
	Status    string
	Priority  string
	Owner     string
	UpdatedAt string
}

type demoNote struct {
	Title     string
	Path      string
	Body      string
	UpdatedAt string
}

type demoFile struct {
	Path    string
	Content string
}

type demoFixtureSet struct {
	WorkspaceID  string
	ProjectName  string
	Tasks        []demoTask
	Notes        []demoNote
	Files        []demoFile
	FixtureLabel string
}

var demoFixtures = map[string]demoFixtureSet{
	"apollo": {
		WorkspaceID: "apollo",
		ProjectName: "Apollo Dashboard Refresh",
		Tasks: []demoTask{
			{TaskID: "APL-101", Title: "Unify dashboard card spacing", Status: "open", Priority: "high", Owner: "maya", UpdatedAt: "2026-03-15T08:30:00Z"},
			{TaskID: "APL-102", Title: "Add route for task summary payload", Status: "open", Priority: "urgent", Owner: "ian", UpdatedAt: "2026-03-15T10:05:00Z"},
			{TaskID: "APL-103", Title: "Review onboarding note examples", Status: "in_review", Priority: "medium", Owner: "zoe", UpdatedAt: "2026-03-14T16:20:00Z"},
		},
		Notes: []demoNote{
			{Title: "Daily sync", Path: "notes/daily-sync.md", Body: "# Daily sync\n\n- Keep `/tasks` payload stable.\n- Dashboard note should link the preview route.\n", UpdatedAt: "2026-03-15T09:00:00Z"},
			{Title: "Open questions", Path: "notes/open-questions.md", Body: "# Open questions\n\n- Should the dashboard note embed route samples?\n- Which tasks need frontend review?\n", UpdatedAt: "2026-03-15T07:45:00Z"},
		},
		Files: []demoFile{
			{Path: "README.md", Content: "# Apollo Dashboard Refresh\n\nThis workspace contains demo project notes and generated artifacts.\n"},
			{Path: "config/routes.json", Content: "{\n  \"base\": \"/api\"\n}\n"},
		},
		FixtureLabel: "project-fixtures-v1",
	},
	"mercury": {
		WorkspaceID: "mercury",
		ProjectName: "Mercury Notes Cleanup",
		Tasks: []demoTask{
			{TaskID: "MRC-201", Title: "Merge duplicate meeting notes", Status: "open", Priority: "medium", Owner: "noah", UpdatedAt: "2026-03-13T14:00:00Z"},
			{TaskID: "MRC-202", Title: "Add task digest route", Status: "open", Priority: "high", Owner: "sara", UpdatedAt: "2026-03-14T12:15:00Z"},
		},
		Notes: []demoNote{
			{Title: "Migration sketch", Path: "notes/migration-sketch.md", Body: "# Migration sketch\n\nConsolidate legacy notes into one summary page.\n", UpdatedAt: "2026-03-14T12:10:00Z"},
		},
		Files: []demoFile{
			{Path: "docs/plan.md", Content: "# Plan\n\n1. Audit notes\n2. Create digest\n3. Expose routes\n"},
		},
		FixtureLabel: "project-fixtures-v1",
	},
}

type materializedWorkspace struct {
	Root string
}

func availableDemoWorkspaces() []string {
	ret := make([]string, 0, len(demoFixtures))
	for workspaceID := range demoFixtures {
		ret = append(ret, workspaceID)
	}
	sort.Strings(ret)
	return ret
}

func demoFixturesForWorkspace(workspaceID string) (demoFixtureSet, error) {
	key := strings.TrimSpace(workspaceID)
	if fixtures, ok := demoFixtures[key]; ok {
		return fixtures, nil
	}
	return demoFixtureSet{}, fmt.Errorf("unknown workspace %q (available: %s)", key, strings.Join(availableDemoWorkspaces(), ", "))
}

func materializeDemoWorkspace(fixtures demoFixtureSet) (*materializedWorkspace, error) {
	root, err := os.MkdirTemp("", "scopedjs-tui-demo-*")
	if err != nil {
		return nil, fmt.Errorf("create temp workspace: %w", err)
	}
	writeFile := func(relPath string, content string) error {
		absPath := filepath.Join(root, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(absPath, []byte(content), 0o644)
	}

	for _, note := range fixtures.Notes {
		if err := writeFile(note.Path, note.Body); err != nil {
			_ = os.RemoveAll(root)
			return nil, fmt.Errorf("write note %s: %w", note.Path, err)
		}
	}
	for _, file := range fixtures.Files {
		if err := writeFile(file.Path, file.Content); err != nil {
			_ = os.RemoveAll(root)
			return nil, fmt.Errorf("write file %s: %w", file.Path, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		_ = os.RemoveAll(root)
		return nil, fmt.Errorf("create artifacts dir: %w", err)
	}
	return &materializedWorkspace{Root: root}, nil
}

func (w *materializedWorkspace) Cleanup() error {
	if w == nil || strings.TrimSpace(w.Root) == "" {
		return nil
	}
	return os.RemoveAll(w.Root)
}
