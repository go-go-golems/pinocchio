package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopeddb"
)

const demoToolName = "query_support_history"

const demoSchemaSQL = `
CREATE TABLE accounts (
  account_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  plan TEXT NOT NULL
);

CREATE TABLE tickets (
  ticket_id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  title TEXT NOT NULL,
  status TEXT NOT NULL,
  priority TEXT NOT NULL,
  opened_at TEXT NOT NULL
);

CREATE TABLE ticket_events (
  event_id TEXT PRIMARY KEY,
  ticket_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at TEXT NOT NULL
);
`

type demoScope struct {
	AccountID string
}

type demoMeta struct {
	AccountID    string
	AccountName  string
	Plan         string
	TicketCount  int
	EventCount   int
	FixtureLabel string
}

func demoDatasetSpec() scopeddb.DatasetSpec[demoScope, demoMeta] {
	return scopeddb.DatasetSpec[demoScope, demoMeta]{
		InMemoryPrefix: "scopeddb_tui_demo",
		SchemaLabel:    "scopeddb TUI demo schema",
		SchemaSQL:      demoSchemaSQL,
		AllowedObjects: []string{"accounts", "tickets", "ticket_events"},
		Tool: scopeddb.ToolDefinitionSpec{
			Name: demoToolName,
			Description: scopeddb.ToolDescription{
				Summary: "Query a scoped read-only SQLite snapshot of support ticket history for a single customer account.",
				StarterQueries: []string{
					"SELECT ticket_id, title, status, priority, opened_at FROM tickets ORDER BY opened_at DESC LIMIT 5",
					"SELECT event_type, created_at, body FROM ticket_events WHERE ticket_id = ? ORDER BY created_at",
				},
				Notes: []string{
					"The snapshot already contains only one customer account.",
					"Join tickets and ticket_events when you need both ticket metadata and event detail.",
				},
			},
			Tags:    []string{"sqlite", "scopeddb", "demo", "support"},
			Version: "1.0.0",
		},
		DefaultQuery: scopeddb.QueryOptions{
			MaxRows:        50,
			MaxColumns:     16,
			MaxCellChars:   400,
			Timeout:        3 * time.Second,
			RequireOrderBy: true,
		},
		Materialize: materializeDemoScope,
	}
}

func materializeDemoScope(ctx context.Context, dst *sql.DB, scope demoScope) (demoMeta, error) {
	fixtures, err := demoFixturesForAccount(scope.AccountID)
	if err != nil {
		return demoMeta{}, err
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if _, err := dst.ExecContext(ctx,
		`INSERT INTO accounts(account_id, name, plan) VALUES (?, ?, ?)`,
		fixtures.Account.AccountID,
		fixtures.Account.Name,
		fixtures.Account.Plan,
	); err != nil {
		return demoMeta{}, fmt.Errorf("insert account: %w", err)
	}

	for _, ticket := range fixtures.Tickets {
		if _, err := dst.ExecContext(ctx,
			`INSERT INTO tickets(ticket_id, account_id, title, status, priority, opened_at) VALUES (?, ?, ?, ?, ?, ?)`,
			ticket.TicketID,
			ticket.AccountID,
			ticket.Title,
			ticket.Status,
			ticket.Priority,
			ticket.OpenedAt,
		); err != nil {
			return demoMeta{}, fmt.Errorf("insert ticket %s: %w", ticket.TicketID, err)
		}
	}

	for _, event := range fixtures.Events {
		if _, err := dst.ExecContext(ctx,
			`INSERT INTO ticket_events(event_id, ticket_id, event_type, body, created_at) VALUES (?, ?, ?, ?, ?)`,
			event.EventID,
			event.TicketID,
			event.EventType,
			event.Body,
			event.CreatedAt,
		); err != nil {
			return demoMeta{}, fmt.Errorf("insert event %s: %w", event.EventID, err)
		}
	}

	return demoMeta{
		AccountID:    fixtures.Account.AccountID,
		AccountName:  fixtures.Account.Name,
		Plan:         fixtures.Account.Plan,
		TicketCount:  len(fixtures.Tickets),
		EventCount:   len(fixtures.Events),
		FixtureLabel: "literal-fixtures-v1",
	}, nil
}

func buildDemoRegistry(ctx context.Context, scope demoScope) (*tools.InMemoryToolRegistry, demoMeta, func() error, error) {
	spec := demoDatasetSpec()
	buildResult, err := scopeddb.BuildInMemory(ctx, spec, scope)
	if err != nil {
		return nil, demoMeta{}, nil, err
	}
	cleanup := buildResult.Cleanup
	registry := tools.NewInMemoryToolRegistry()
	if err := scopeddb.RegisterPrebuilt(registry, spec, buildResult.DB, spec.DefaultQuery); err != nil {
		if cleanup != nil {
			_ = cleanup()
		}
		return nil, demoMeta{}, nil, err
	}
	return registry, buildResult.Meta, cleanup, nil
}

func systemPrompt(meta demoMeta) string {
	return strings.TrimSpace(fmt.Sprintf(`
You are a support operations assistant.

The available SQL tool is scoped to account %s (%s plan). Use it when the user asks for ticket history, recent incidents, open issues, event timelines, or summaries grounded in account data.

Prefer short SELECT queries with explicit ORDER BY clauses. After reading the tool output, answer in plain English and cite concrete ticket ids when useful.
`, meta.AccountName, meta.Plan))
}
