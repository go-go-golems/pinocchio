package main

import (
	"context"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopeddb"
)

func TestBuildDemoRegistryMaterializesFixtures(t *testing.T) {
	t.Parallel()

	registry, meta, cleanup, err := buildDemoRegistry(context.Background(), demoScope{AccountID: "acme-co"})
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
	if meta.AccountName != "Acme Co" {
		t.Fatalf("unexpected account name: %q", meta.AccountName)
	}
	if meta.TicketCount != 3 {
		t.Fatalf("unexpected ticket count: %d", meta.TicketCount)
	}
	if meta.EventCount != 8 {
		t.Fatalf("unexpected event count: %d", meta.EventCount)
	}
}

func TestDemoDatasetQueryRunnerReadsScopedData(t *testing.T) {
	t.Parallel()

	spec := demoDatasetSpec()
	buildResult, err := scopeddb.BuildInMemory(context.Background(), spec, demoScope{AccountID: "northwind"})
	if err != nil {
		t.Fatalf("BuildInMemory returned error: %v", err)
	}
	defer func() { _ = buildResult.Cleanup() }()

	runner, err := scopeddb.NewQueryRunner(buildResult.DB, scopeddb.AllowedObjectMap(spec.AllowedObjects), spec.DefaultQuery)
	if err != nil {
		t.Fatalf("NewQueryRunner returned error: %v", err)
	}

	out, err := runner.Run(context.Background(), scopeddb.QueryInput{
		SQL: `SELECT ticket_id, status FROM tickets ORDER BY opened_at DESC LIMIT 2`,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("expected no query error, got %q", out.Error)
	}
	if out.Count != 2 {
		t.Fatalf("unexpected row count: %d", out.Count)
	}
	if got := out.Rows[0]["ticket_id"]; got != "NW-201" {
		t.Fatalf("unexpected first row ticket_id: %v", got)
	}
}

func TestDemoFixturesForUnknownAccount(t *testing.T) {
	t.Parallel()

	_, err := demoFixturesForAccount("missing")
	if err == nil {
		t.Fatalf("expected error for unknown account")
	}
}
