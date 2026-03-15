package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	chatstyle "github.com/go-go-golems/bobatea/pkg/timeline/chatstyle"
	base_renderers "github.com/go-go-golems/bobatea/pkg/timeline/renderers"
	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopeddb"
	"github.com/muesli/termenv"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

func registerDemoRenderers(r *timeline.Registry) {
	r.RegisterModelFactory(base_renderers.NewLLMTextFactory())
	r.RegisterModelFactory(base_renderers.PlainFactory{})
	r.RegisterModelFactory(newSQLToolCallFactory(demoToolName))
	r.RegisterModelFactory(newQueryResultTableFactory())
	r.RegisterModelFactory(base_renderers.LogEventFactory{})
}

type sqlToolCallModel struct {
	toolName string
	name     string
	inputRaw string
	width    int
	selected bool
	focused  bool
	style    *chatstyle.Style
	renderer *glamour.TermRenderer
}

func (m *sqlToolCallModel) Init() tea.Cmd { return nil }

func (m *sqlToolCallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case timeline.EntitySelectedMsg:
		m.selected = true
	case timeline.EntityUnselectedMsg:
		m.selected = false
		m.focused = false
	case timeline.EntityPropsUpdatedMsg:
		if v.Patch != nil {
			m.onProps(v.Patch)
		}
	case timeline.EntitySetSizeMsg:
		m.width = v.Width
		return m, nil
	case timeline.EntityFocusMsg:
		m.focused = true
	case timeline.EntityBlurMsg:
		m.focused = false
	}
	return m, nil
}

func (m *sqlToolCallModel) View() string {
	sty := demoChatStyle(m.style, m.selected, m.focused)
	body := "-> " + strings.TrimSpace(m.name)
	mdBody := buildToolCallMarkdown(m.toolName, m.name, m.inputRaw)
	if mdBody != "" {
		body += "\n\n" + mdBody
	}
	return renderMarkdownBody(sty, m.width, m.renderer, body)
}

func (m *sqlToolCallModel) onProps(patch map[string]any) {
	if v, ok := patch["name"].(string); ok {
		m.name = v
	}
	if v, ok := patch["input"].(string); ok {
		m.inputRaw = strings.TrimSpace(v)
	}
}

type sqlToolCallFactory struct {
	toolName string
	renderer *glamour.TermRenderer
}

func newSQLToolCallFactory(toolName string) *sqlToolCallFactory {
	return &sqlToolCallFactory{
		toolName: toolName,
		renderer: newGlamourRenderer(),
	}
}

func (f *sqlToolCallFactory) Key() string  { return "renderer.tool_call.sql_scopeddb.v1" }
func (f *sqlToolCallFactory) Kind() string { return "tool_call" }
func (f *sqlToolCallFactory) NewEntityModel(initialProps map[string]any) timeline.EntityModel {
	m := &sqlToolCallModel{toolName: f.toolName, renderer: f.renderer}
	m.onProps(initialProps)
	return m
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

	var anyv any
	if (strings.HasPrefix(inputRaw, "{") || strings.HasPrefix(inputRaw, "[")) && json.Unmarshal([]byte(inputRaw), &anyv) == nil {
		if b, err := yaml.Marshal(anyv); err == nil {
			return "```yaml\n" + strings.TrimSpace(string(b)) + "\n```"
		}
	}
	return "```text\n" + inputRaw + "\n```"
}

type queryResultTableModel struct {
	rawResult string
	md        string
	width     int
	selected  bool
	focused   bool
	style     *chatstyle.Style
	renderer  *glamour.TermRenderer
}

func (m *queryResultTableModel) Init() tea.Cmd { return nil }

func (m *queryResultTableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case timeline.EntitySelectedMsg:
		m.selected = true
	case timeline.EntityUnselectedMsg:
		m.selected = false
		m.focused = false
	case timeline.EntityPropsUpdatedMsg:
		if v.Patch != nil {
			m.onProps(v.Patch)
		}
	case timeline.EntitySetSizeMsg:
		m.width = v.Width
		if strings.TrimSpace(m.rawResult) != "" {
			m.md = formatQueryResultMarkdown(m.rawResult, m.width)
		}
		return m, nil
	case timeline.EntityFocusMsg:
		m.focused = true
	case timeline.EntityBlurMsg:
		m.focused = false
	}
	return m, nil
}

func (m *queryResultTableModel) View() string {
	sty := demoChatStyle(m.style, m.selected, m.focused)
	body := strings.TrimSpace(m.md)
	if body == "" {
		body = strings.TrimSpace(m.rawResult)
	}
	return renderMarkdownBody(sty, m.width, m.renderer, body)
}

func (m *queryResultTableModel) onProps(patch map[string]any) {
	if v, ok := patch["result"].(string); ok {
		m.rawResult = strings.TrimSpace(v)
		m.md = formatQueryResultMarkdown(m.rawResult, m.width)
	}
}

type queryResultTableFactory struct{ renderer *glamour.TermRenderer }

func newQueryResultTableFactory() *queryResultTableFactory {
	return &queryResultTableFactory{renderer: newGlamourRenderer()}
}

func (f *queryResultTableFactory) Key() string  { return "renderer.tool_call_result.scopeddb_table.v1" }
func (f *queryResultTableFactory) Kind() string { return "tool_call_result" }
func (f *queryResultTableFactory) NewEntityModel(initialProps map[string]any) timeline.EntityModel {
	m := &queryResultTableModel{renderer: f.renderer}
	m.onProps(initialProps)
	return m
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
		widths[i] = clampInt(runeLen(col), minColWidth, maxColWidth)
	}
	for _, row := range displayRows {
		for i, col := range cols {
			widths[i] = clampInt(maxInt(widths[i], runeLen(stringifyCell(row[col]))), minColWidth, maxColWidth)
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
			values = append(values, stringifyCell(row[col]))
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
			cell = truncateRunes(cell, widths[i])
		}
		parts = append(parts, padRight(cell, widths[i]))
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

func renderMarkdownBody(sty lipgloss.Style, width int, renderer *glamour.TermRenderer, body string) string {
	rendered := strings.TrimSpace(body)
	if renderer != nil && rendered != "" {
		if out, err := renderer.Render(rendered + "\n"); err == nil {
			rendered = strings.TrimSpace(out)
		}
	}
	return sty.Width(maxInt(1, width-sty.GetHorizontalPadding())).Render(rendered)
}

func demoChatStyle(style *chatstyle.Style, selected bool, focused bool) lipgloss.Style {
	if style == nil {
		style = chatstyle.DefaultStyles()
	}
	sty := style.UnselectedMessage
	if selected {
		sty = style.SelectedMessage
	}
	if focused && !selected {
		sty = style.FocusedMessage
	}
	return sty
}

func newGlamourRenderer() *glamour.TermRenderer {
	style := "light"
	if !stdoutIsTerminal() {
		style = "notty"
	} else if termenv.HasDarkBackground() {
		style = "dark"
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create glamour renderer")
		return nil
	}
	return r
}

func stdoutIsTerminal() bool {
	fd := os.Stdout.Fd()
	if fd > uintptr(math.MaxInt) {
		return false
	}
	return term.IsTerminal(int(fd))
}

func stringifyCell(v any) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		b, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(b)
	}
}

func padRight(v string, width int) string {
	diff := width - runeLen(v)
	if diff <= 0 {
		return v
	}
	return v + strings.Repeat(" ", diff)
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func runeLen(v string) int {
	return utf8.RuneCountInString(v)
}

func truncateRunes(v string, width int) string {
	if width <= 0 || runeLen(v) <= width {
		return v
	}
	if width <= 1 {
		return "…"
	}
	rs := []rune(v)
	return string(rs[:width-1]) + "…"
}
