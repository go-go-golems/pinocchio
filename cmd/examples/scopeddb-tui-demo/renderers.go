package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/go-go-golems/bobatea/pkg/timeline"
	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopeddb"
	"github.com/go-go-golems/pinocchio/cmd/examples/internal/demorender"
	"gopkg.in/yaml.v3"
)

func registerDemoRenderers(r *timeline.Registry) {
	demorender.RegisterBaseRenderers(r,
		demorender.NewToolCallFactory("renderer.tool_call.sql_scopeddb.v1", demoToolName, buildToolCallMarkdown),
		demorender.NewResultFactory("renderer.tool_call_result.scopeddb_table.v1", formatQueryResultMarkdown),
	)
}

func buildToolCallMarkdown(expectedToolName, actualToolName, inputRaw string) string {
	actualToolName = strings.TrimSpace(actualToolName)
	inputRaw = strings.TrimSpace(inputRaw)
	if inputRaw == "" {
		return ""
	}
	if actualToolName == expectedToolName {
		var in scopeddb.QueryInput
		if err := json.Unmarshal([]byte(inputRaw), &in); err == nil && strings.TrimSpace(in.SQL) != "" {
			md := "```sql\n" + strings.TrimSpace(in.SQL) + "\n```"
			if len(in.Params) > 0 {
				if b, err := yaml.Marshal(in.Params); err == nil {
					md += "\n\nparams:\n```yaml\n" + strings.TrimSpace(string(b)) + "\n```"
				}
			}
			return md
		}
	}
	return demorender.FencedAny(inputRaw)
}

func formatQueryResultMarkdown(raw string, width int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var out scopeddb.QueryOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return raw
	}
	if out.Error != "" {
		return "Error: " + out.Error
	}
	if len(out.Columns) == 0 {
		return raw
	}
	return renderMarkdownTable(out.Columns, out.Rows, out.Count, out.Truncated, width)
}

func renderMarkdownTable(columns []string, rows []map[string]any, count int, truncated bool, width int) string {
	const (
		minColWidth = 6
		maxColWidth = 44
		maxRowsShow = 20
	)

	cols := make([]string, 0, len(columns))
	for _, col := range columns {
		col = strings.TrimSpace(col)
		if col == "" {
			col = "(col)"
		}
		cols = append(cols, col)
	}

	displayRows := rows
	if len(displayRows) > maxRowsShow {
		displayRows = displayRows[:maxRowsShow]
		truncated = true
	}

	widths := make([]int, len(cols))
	for i, col := range cols {
		widths[i] = demorender.ClampInt(demorender.RuneLen(col), minColWidth, maxColWidth)
	}
	for _, row := range displayRows {
		for i, col := range cols {
			widths[i] = demorender.ClampInt(demorender.MaxInt(widths[i], demorender.RuneLen(demorender.StringifyCell(row[col]))), minColWidth, maxColWidth)
		}
	}

	maxWidth := width
	if maxWidth <= 0 {
		maxWidth = 100
	}
	visibleCols, visibleWidths, omittedCols := fitColumnsToWidth(cols, widths, maxWidth)

	var b strings.Builder
	fmt.Fprintf(&b, "rows=%d", count)
	if truncated {
		b.WriteString(" (truncated)")
	}
	b.WriteString("\n")
	b.WriteString(markdownTableLine(visibleCols, visibleWidths, nil, false))
	b.WriteString("\n")
	b.WriteString(markdownTableLine(visibleCols, visibleWidths, nil, true))
	b.WriteString("\n")

	for _, row := range displayRows {
		values := make([]string, 0, len(visibleCols))
		for _, col := range visibleCols {
			values = append(values, demorender.StringifyCell(row[col]))
		}
		b.WriteString(markdownTableLine(visibleCols, visibleWidths, values, false))
		b.WriteString("\n")
	}

	if len(omittedCols) > 0 {
		sort.Strings(omittedCols)
		fmt.Fprintf(&b, "... omitted %d columns: %s\n", len(omittedCols), strings.Join(omittedCols, ", "))
	}
	return strings.TrimRight(b.String(), "\n")
}

func markdownTableLine(cols []string, widths []int, values []string, separator bool) string {
	parts := make([]string, 0, len(cols))
	for i := range cols {
		cell := cols[i]
		if values != nil {
			cell = values[i]
		}
		if separator {
			cell = strings.Repeat("-", widths[i])
		} else {
			cell = demorender.TruncateRunes(cell, widths[i])
		}
		parts = append(parts, demorender.PadRight(cell, widths[i]))
	}
	return "| " + strings.Join(parts, " | ") + " |"
}

func fitColumnsToWidth(cols []string, widths []int, avail int) ([]string, []int, []string) {
	visibleCols := append([]string{}, cols...)
	visibleWidths := append([]int{}, widths...)
	omitted := []string{}

	for markdownTableWidth(visibleWidths) > avail && canShrink(visibleWidths) {
		i := indexOfWidestShrinkable(visibleWidths)
		if i < 0 {
			break
		}
		visibleWidths[i]--
	}

	for len(visibleCols) > 1 && markdownTableWidth(visibleWidths) > avail {
		last := len(visibleCols) - 1
		omitted = append(omitted, visibleCols[last])
		visibleCols = visibleCols[:last]
		visibleWidths = visibleWidths[:last]
		for markdownTableWidth(visibleWidths) > avail && canShrink(visibleWidths) {
			i := indexOfWidestShrinkable(visibleWidths)
			if i < 0 {
				break
			}
			visibleWidths[i]--
		}
	}
	return visibleCols, visibleWidths, omitted
}

func markdownTableWidth(widths []int) int {
	if len(widths) == 0 {
		return 0
	}
	total := 4
	for _, width := range widths {
		total += width + 3
	}
	return total
}

func canShrink(widths []int) bool {
	for _, width := range widths {
		if width > 6 {
			return true
		}
	}
	return false
}

func indexOfWidestShrinkable(widths []int) int {
	bestIdx := -1
	bestWidth := -1
	for i, width := range widths {
		if width <= 6 {
			continue
		}
		if width > bestWidth {
			bestWidth = width
			bestIdx = i
		}
	}
	return bestIdx
}
