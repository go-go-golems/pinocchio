package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	enginefactory "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/pinocchio/cmd/examples/internal/tuidemo"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func main() {
	var (
		workspaceID       string
		profileSlug       string
		profileRegistries string
		logLevel          string
		listWorkspaces    bool
	)

	root := &cobra.Command{
		Use:   "scopedjs-tui-demo",
		Short: "Bubble Tea demo for a scoped JavaScript project-ops tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			zerolog.TimeFieldFormat = time.StampMilli
			if parsed, err := zerolog.ParseLevel(strings.TrimSpace(logLevel)); err == nil {
				zerolog.SetGlobalLevel(parsed)
			}

			if listWorkspaces {
				for _, workspace := range availableDemoWorkspaces() {
					fmt.Println(workspace)
				}
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stepSettings, closeRuntime, err := tuidemo.ResolveStepSettings(ctx, profileSlug, profileRegistries)
			if err != nil {
				return errors.Wrap(err, "resolve step settings")
			}
			if closeRuntime != nil {
				defer closeRuntime()
			}

			engineInstance, err := enginefactory.NewEngineFromStepSettings(stepSettings)
			if err != nil {
				return errors.Wrap(err, "create engine")
			}

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
	}

	root.Flags().StringVar(&workspaceID, "workspace", "apollo", "Fixture workspace to scope the demo runtime to")
	root.Flags().StringVar(&profileSlug, "profile", "", "Optional profile slug to resolve from profile registries")
	root.Flags().StringVar(&profileRegistries, "profile-registries", "", "Optional comma-separated profile registry sources (yaml/sqlite/sqlite-dsn)")
	root.Flags().StringVar(&logLevel, "log-level", "info", "Log level (trace|debug|info|warn|error)")
	root.Flags().BoolVar(&listWorkspaces, "list-workspaces", false, "List available fake workspaces and exit")

	if err := root.Execute(); err != nil {
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
