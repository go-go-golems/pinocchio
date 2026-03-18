package profileswitch

import (
	"fmt"
	"strings"
)

// DiffSummary describes the changes between two resolved profiles.
type DiffSummary struct {
	From    string
	To      string
	Changes []string // Human-readable change descriptions.
}

// ProfileDiff computes a summary of changes between two resolved profiles.
func ProfileDiff(from, to Resolved) DiffSummary {
	d := DiffSummary{
		From: from.ProfileSlug.String(),
		To:   to.ProfileSlug.String(),
	}

	fromModel := effectiveModel(from)
	toModel := effectiveModel(to)
	if fromModel != toModel {
		d.Changes = append(d.Changes, fmt.Sprintf("model: %s → %s", fromModel, toModel))
	}

	fromTemp := effectiveTemp(from)
	toTemp := effectiveTemp(to)
	if fromTemp != toTemp {
		d.Changes = append(d.Changes, fmt.Sprintf("temperature: %s → %s", fromTemp, toTemp))
	}

	if len(d.Changes) == 0 {
		d.Changes = append(d.Changes, "no visible changes")
	}

	return d
}

// String returns a formatted multi-line summary.
func (d DiffSummary) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s → %s\n", d.From, d.To)
	for _, c := range d.Changes {
		fmt.Fprintf(&sb, "  • %s\n", c)
	}
	return sb.String()
}

func effectiveModel(r Resolved) string {
	if s := r.InferenceSettings; s != nil && s.Chat != nil && s.Chat.Engine != nil {
		return *s.Chat.Engine
	}
	return "(default)"
}

func effectiveTemp(r Resolved) string {
	if s := r.InferenceSettings; s != nil && s.Chat != nil && s.Chat.Temperature != nil {
		return fmt.Sprintf("%.1f", *s.Chat.Temperature)
	}
	return "(default)"
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
