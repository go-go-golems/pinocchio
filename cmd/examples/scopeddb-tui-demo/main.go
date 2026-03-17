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
		Use:            "scopeddb-tui-demo",
		Short:          "Bubble Tea demo for a scopeddb-backed support history tool",
		FixtureFlag:    "account",
		FixtureDefault: "acme-co",
		FixtureUsage:   "Fixture account to scope the demo database to",
		ListFlag:       "list-accounts",
		ListUsage:      "List available fake accounts and exit",
		ListValues:     availableDemoAccounts,
		Run: func(ctx context.Context, accountID string, engineInstance engine.Engine) error {
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
	}); err != nil {
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
