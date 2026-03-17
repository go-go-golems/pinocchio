package main

import (
	"context"
	"fmt"
	"os"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/pinocchio/cmd/examples/internal/tuidemo"
	"github.com/rs/zerolog/log"
)

func main() {
	if err := tuidemo.ExecuteCLI(tuidemo.CLISpec{
		Use:            "scopedjs-tui-demo",
		Short:          "Bubble Tea demo for a scoped JavaScript project-ops tool",
		FixtureFlag:    "workspace",
		FixtureDefault: "apollo",
		FixtureUsage:   "Fixture workspace to scope the demo runtime to",
		ListFlag:       "list-workspaces",
		ListUsage:      "List available fake workspaces and exit",
		ListValues:     availableDemoWorkspaces,
		Run: func(ctx context.Context, workspaceID string, engineInstance engine.Engine) error {
			registry, meta, cleanup, err := buildDemoRegistry(ctx, workspaceID)
			if err != nil {
				return err
			}
			defer func() {
				if cleanup != nil {
					_ = cleanup()
				}
			}()

			log.Info().
				Str("workspace_id", meta.WorkspaceID).
				Str("project_name", meta.ProjectName).
				Int("file_count", meta.FileCount).
				Int("task_count", meta.TaskCount).
				Int("note_count", meta.NoteCount).
				Str("fixture_label", meta.FixtureLabel).
				Msg("loaded scopedjs demo workspace")

			return tuidemo.RunToolLoopDemo(ctx, tuidemo.RunSpec{
				Title:            "scopedjs project ops demo",
				Engine:           engineInstance,
				Registry:         registry,
				SystemPrompt:     systemPrompt(meta),
				TimelineRegister: registerDemoRenderers,
				StatusBarView:    makeStatusBar(meta),
			})
		},
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func makeStatusBar(meta demoMeta) func() string {
	return tuidemo.NewStatusBarView([]tuidemo.StatusPart{
		{Label: "workspace", Value: meta.ProjectName},
		{Label: "files", Value: fmt.Sprintf("%d", meta.FileCount)},
		{Label: "tasks", Value: fmt.Sprintf("%d", meta.TaskCount)},
		{Label: "notes", Value: fmt.Sprintf("%d", meta.NoteCount)},
	}, "try: summarize open tasks / create a dashboard note / register /tasks")
}
