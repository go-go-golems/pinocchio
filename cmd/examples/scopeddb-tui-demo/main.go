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
		accountID         string
		profileSlug       string
		profileRegistries string
		logLevel          string
		listAccounts      bool
	)

	root := &cobra.Command{
		Use:   "scopeddb-tui-demo",
		Short: "Bubble Tea demo for a scopeddb-backed support history tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			zerolog.TimeFieldFormat = time.StampMilli
			if parsed, err := zerolog.ParseLevel(strings.TrimSpace(logLevel)); err == nil {
				zerolog.SetGlobalLevel(parsed)
			}

			if listAccounts {
				for _, account := range availableDemoAccounts() {
					fmt.Println(account)
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

			registry, meta, cleanup, err := buildDemoRegistry(ctx, demoScope{AccountID: accountID})
			if err != nil {
				return err
			}
			defer func() {
				if cleanup != nil {
					_ = cleanup()
				}
			}()

			log.Info().
				Str("account_id", meta.AccountID).
				Str("account_name", meta.AccountName).
				Str("plan", meta.Plan).
				Int("ticket_count", meta.TicketCount).
				Int("event_count", meta.EventCount).
				Str("fixture_label", meta.FixtureLabel).
				Msg("loaded scopeddb demo snapshot")

			return tuidemo.RunToolLoopDemo(ctx, tuidemo.RunSpec{
				Title:            "scopeddb support history demo",
				Engine:           engineInstance,
				Registry:         registry,
				SystemPrompt:     systemPrompt(meta),
				TimelineRegister: registerDemoRenderers,
				StatusBarView:    makeStatusBar(meta),
			})
		},
	}

	root.Flags().StringVar(&accountID, "account", "acme-co", "Fixture account to scope the demo database to")
	root.Flags().StringVar(&profileSlug, "profile", "", "Optional profile slug to resolve from profile registries")
	root.Flags().StringVar(&profileRegistries, "profile-registries", "", "Optional comma-separated profile registry sources (yaml/sqlite/sqlite-dsn)")
	root.Flags().StringVar(&logLevel, "log-level", "info", "Log level (trace|debug|info|warn|error)")
	root.Flags().BoolVar(&listAccounts, "list-accounts", false, "List available fake accounts and exit")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func makeStatusBar(meta demoMeta) func() string {
	return tuidemo.NewStatusBarView([]tuidemo.StatusPart{
		{Label: "account", Value: meta.AccountName},
		{Label: "plan", Value: meta.Plan},
		{Label: "tickets", Value: fmt.Sprintf("%d", meta.TicketCount)},
		{Label: "events", Value: fmt.Sprintf("%d", meta.EventCount)},
	}, "try: list open tickets / show latest event timeline")
}
