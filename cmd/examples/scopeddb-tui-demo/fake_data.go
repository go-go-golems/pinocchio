package main

import (
	"fmt"
	"sort"
	"strings"
)

type demoAccount struct {
	AccountID string
	Name      string
	Plan      string
}

type demoTicket struct {
	TicketID  string
	AccountID string
	Title     string
	Status    string
	Priority  string
	OpenedAt  string
}

type demoTicketEvent struct {
	EventID   string
	TicketID  string
	EventType string
	Body      string
	CreatedAt string
}

type demoFixtureSet struct {
	Account demoAccount
	Tickets []demoTicket
	Events  []demoTicketEvent
}

var demoFixtures = map[string]demoFixtureSet{
	"acme-co": {
		Account: demoAccount{AccountID: "acme-co", Name: "Acme Co", Plan: "enterprise"},
		Tickets: []demoTicket{
			{TicketID: "ACME-101", AccountID: "acme-co", Title: "EU export sync stalls overnight", Status: "open", Priority: "high", OpenedAt: "2026-03-11T09:15:00Z"},
			{TicketID: "ACME-102", AccountID: "acme-co", Title: "Billing webhook retries duplicate invoices", Status: "open", Priority: "urgent", OpenedAt: "2026-03-12T14:40:00Z"},
			{TicketID: "ACME-103", AccountID: "acme-co", Title: "SSO rollout missing invited contractors", Status: "closed", Priority: "medium", OpenedAt: "2026-03-08T08:00:00Z"},
		},
		Events: []demoTicketEvent{
			{EventID: "evt-acme-1", TicketID: "ACME-101", EventType: "opened", Body: "Customer reports the nightly export job hanging after 02:00 UTC.", CreatedAt: "2026-03-11T09:15:00Z"},
			{EventID: "evt-acme-2", TicketID: "ACME-101", EventType: "triage_note", Body: "Ops reproduced timeout when batch size exceeds 2k rows.", CreatedAt: "2026-03-11T11:05:00Z"},
			{EventID: "evt-acme-3", TicketID: "ACME-101", EventType: "status_update", Body: "Patch under review; customer asked for ETA before Monday.", CreatedAt: "2026-03-12T16:20:00Z"},
			{EventID: "evt-acme-4", TicketID: "ACME-102", EventType: "opened", Body: "Finance team noticed duplicate invoices on webhook retries.", CreatedAt: "2026-03-12T14:40:00Z"},
			{EventID: "evt-acme-5", TicketID: "ACME-102", EventType: "customer_quote", Body: "If this retries again during close we will pause the integration.", CreatedAt: "2026-03-12T15:12:00Z"},
			{EventID: "evt-acme-6", TicketID: "ACME-102", EventType: "engineering_note", Body: "Idempotency key missing from retry path in legacy worker.", CreatedAt: "2026-03-13T10:00:00Z"},
			{EventID: "evt-acme-7", TicketID: "ACME-103", EventType: "opened", Body: "Contractor accounts absent after SSO migration.", CreatedAt: "2026-03-08T08:00:00Z"},
			{EventID: "evt-acme-8", TicketID: "ACME-103", EventType: "resolved", Body: "Backfill completed and access restored.", CreatedAt: "2026-03-09T18:25:00Z"},
		},
	},
	"northwind": {
		Account: demoAccount{AccountID: "northwind", Name: "Northwind Traders", Plan: "growth"},
		Tickets: []demoTicket{
			{TicketID: "NW-201", AccountID: "northwind", Title: "Warehouse tablet sessions expire too quickly", Status: "open", Priority: "medium", OpenedAt: "2026-03-10T12:05:00Z"},
			{TicketID: "NW-202", AccountID: "northwind", Title: "CSV import rejects quoted commas in address field", Status: "closed", Priority: "low", OpenedAt: "2026-03-07T07:30:00Z"},
		},
		Events: []demoTicketEvent{
			{EventID: "evt-nw-1", TicketID: "NW-201", EventType: "opened", Body: "Tablet sign-in loop returns to login after 15 minutes.", CreatedAt: "2026-03-10T12:05:00Z"},
			{EventID: "evt-nw-2", TicketID: "NW-201", EventType: "triage_note", Body: "Session TTL differs between handheld and web clients.", CreatedAt: "2026-03-10T13:15:00Z"},
			{EventID: "evt-nw-3", TicketID: "NW-202", EventType: "opened", Body: "Importer splits addresses when the field contains embedded commas.", CreatedAt: "2026-03-07T07:30:00Z"},
			{EventID: "evt-nw-4", TicketID: "NW-202", EventType: "resolved", Body: "CSV parser upgraded to RFC4180-compliant path.", CreatedAt: "2026-03-08T09:45:00Z"},
		},
	},
	"globex": {
		Account: demoAccount{AccountID: "globex", Name: "Globex", Plan: "starter"},
		Tickets: []demoTicket{
			{TicketID: "GBX-301", AccountID: "globex", Title: "Need sample SQL for KPI dashboard", Status: "open", Priority: "low", OpenedAt: "2026-03-13T08:10:00Z"},
		},
		Events: []demoTicketEvent{
			{EventID: "evt-gbx-1", TicketID: "GBX-301", EventType: "opened", Body: "Customer asks for starter queries to build their first dashboard.", CreatedAt: "2026-03-13T08:10:00Z"},
			{EventID: "evt-gbx-2", TicketID: "GBX-301", EventType: "status_update", Body: "Support suggested using tickets by priority and age buckets.", CreatedAt: "2026-03-13T09:00:00Z"},
		},
	},
}

func availableDemoAccounts() []string {
	ret := make([]string, 0, len(demoFixtures))
	for accountID := range demoFixtures {
		ret = append(ret, accountID)
	}
	sort.Strings(ret)
	return ret
}

func demoFixturesForAccount(accountID string) (demoFixtureSet, error) {
	key := strings.TrimSpace(accountID)
	fixtures, ok := demoFixtures[key]
	if ok {
		return fixtures, nil
	}
	return demoFixtureSet{}, fmt.Errorf("unknown account %q (available: %s)", key, strings.Join(availableDemoAccounts(), ", "))
}
