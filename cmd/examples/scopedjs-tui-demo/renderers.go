package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	chatstyle "github.com/go-go-golems/bobatea/pkg/timeline/chatstyle"
	base_renderers "github.com/go-go-golems/bobatea/pkg/timeline/renderers"
	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
	"github.com/muesli/termenv"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

func registerDemoRenderers(r *timeline.Registry) {
	r.RegisterModelFactory(base_renderers.NewLLMTextFactory())
	r.RegisterModelFactory(base_renderers.PlainFactory{})
	r.RegisterModelFactory(newEvalToolCallFactory(demoToolName))
	r.RegisterModelFactory(newEvalResultFactory())
	r.RegisterModelFactory(base_renderers.LogEventFactory{})
}

type evalToolCallModel struct {
	toolName string
	name     string
	inputRaw string
	width    int
	selected bool
	focused  bool
	style    *chatstyle.Style
	renderer *glamour.TermRenderer
}

func (m *evalToolCallModel) Init() tea.Cmd { return nil }

func (m *evalToolCallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *evalToolCallModel) View() string {
	sty := demoChatStyle(m.style, m.selected, m.focused)
	body := "-> " + strings.TrimSpace(m.name)
	mdBody := buildEvalToolCallMarkdown(m.toolName, m.name, m.inputRaw)
	if mdBody != "" {
		body += "\n\n" + mdBody
	}
	return renderMarkdownBody(sty, m.width, m.renderer, body)
}

func (m *evalToolCallModel) onProps(patch map[string]any) {
	if v, ok := patch["name"].(string); ok {
		m.name = v
	}
	if v, ok := patch["input"].(string); ok {
		m.inputRaw = strings.TrimSpace(v)
	}
}

type evalToolCallFactory struct {
	toolName string
	renderer *glamour.TermRenderer
}

func newEvalToolCallFactory(toolName string) *evalToolCallFactory {
	return &evalToolCallFactory{
		toolName: toolName,
		renderer: newGlamourRenderer(),
	}
}

func (f *evalToolCallFactory) Key() string  { return "renderer.tool_call.scopedjs_eval.v1" }
func (f *evalToolCallFactory) Kind() string { return "tool_call" }
func (f *evalToolCallFactory) NewEntityModel(initialProps map[string]any) timeline.EntityModel {
	m := &evalToolCallModel{toolName: f.toolName, renderer: f.renderer}
	m.onProps(initialProps)
	return m
}

type evalResultModel struct {
	rawResult string
	md        string
	width     int
	selected  bool
	focused   bool
	style     *chatstyle.Style
	renderer  *glamour.TermRenderer
}

func (m *evalResultModel) Init() tea.Cmd { return nil }

func (m *evalResultModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.md = formatEvalResultMarkdown(m.rawResult)
		}
		return m, nil
	case timeline.EntityFocusMsg:
		m.focused = true
	case timeline.EntityBlurMsg:
		m.focused = false
	}
	return m, nil
}

func (m *evalResultModel) View() string {
	sty := demoChatStyle(m.style, m.selected, m.focused)
	body := strings.TrimSpace(m.md)
	if body == "" {
		body = strings.TrimSpace(m.rawResult)
	}
	return renderMarkdownBody(sty, m.width, m.renderer, body)
}

func (m *evalResultModel) onProps(patch map[string]any) {
	if v, ok := patch["result"].(string); ok {
		m.rawResult = strings.TrimSpace(v)
		m.md = formatEvalResultMarkdown(m.rawResult)
	}
}

type evalResultFactory struct{ renderer *glamour.TermRenderer }

func newEvalResultFactory() *evalResultFactory {
	return &evalResultFactory{renderer: newGlamourRenderer()}
}

func (f *evalResultFactory) Key() string  { return "renderer.tool_call_result.scopedjs_eval.v1" }
func (f *evalResultFactory) Kind() string { return "tool_call_result" }
func (f *evalResultFactory) NewEntityModel(initialProps map[string]any) timeline.EntityModel {
	m := &evalResultModel{renderer: f.renderer}
	m.onProps(initialProps)
	return m
}

func buildEvalToolCallMarkdown(expectedToolName, actualToolName, inputRaw string) string {
	actualToolName = strings.TrimSpace(actualToolName)
	inputRaw = strings.TrimSpace(inputRaw)
	if inputRaw == "" {
		return ""
	}
	if actualToolName == expectedToolName {
		var in scopedjs.EvalInput
		if err := json.Unmarshal([]byte(inputRaw), &in); err == nil && strings.TrimSpace(in.Code) != "" {
			var b strings.Builder
			b.WriteString("```js\n")
			b.WriteString(strings.TrimSpace(in.Code))
			b.WriteString("\n```")
			if len(in.Input) > 0 {
				if y, err := yaml.Marshal(in.Input); err == nil {
					b.WriteString("\n\ninput:\n```yaml\n")
					b.WriteString(strings.TrimSpace(string(y)))
					b.WriteString("\n```")
				}
			}
			return b.String()
		}
	}
	return fencedAny(inputRaw)
}

func formatEvalResultMarkdown(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var out scopedjs.EvalOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return raw
	}

	parts := make([]string, 0, 4)
	if out.Error != "" {
		parts = append(parts, "**Error**\n\n```text\n"+strings.TrimSpace(out.Error)+"\n```")
	}
	if len(out.Console) > 0 {
		lines := make([]string, 0, len(out.Console))
		for _, line := range out.Console {
			lines = append(lines, "["+strings.TrimSpace(line.Level)+"] "+strings.TrimSpace(line.Text))
		}
		parts = append(parts, "console:\n```text\n"+strings.Join(lines, "\n")+"\n```")
	}
	if out.Result != nil {
		parts = append(parts, formatEvalResultValue(out.Result))
	}
	if out.DurationMs > 0 {
		parts = append(parts, fmt.Sprintf("duration: `%dms`", out.DurationMs))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func formatEvalResultValue(v any) string {
	switch typed := v.(type) {
	case map[string]any:
		return formatEvalResultMap(typed)
	default:
		if y, err := yaml.Marshal(typed); err == nil {
			return "result:\n```yaml\n" + strings.TrimSpace(string(y)) + "\n```"
		}
		return "result:\n" + fencedAny(v)
	}
}

func formatEvalResultMap(m map[string]any) string {
	lines := []string{"result:"}

	if path, ok := stringField(m, "summaryPath"); ok {
		lines = append(lines, "- summaryPath: `"+path+"`")
	}
	if path, ok := stringField(m, "previewPath"); ok {
		lines = append(lines, "- previewPath: `"+path+"`")
	}
	if note, ok := mapField(m, "note"); ok {
		title, _ := stringField(note, "title")
		path, _ := stringField(note, "path")
		if title != "" || path != "" {
			lines = append(lines, fmt.Sprintf("- note: `%s` (%s)", path, title))
		}
	}
	if routes := routeLines(m["routes"]); len(routes) > 0 {
		lines = append(lines, "- routes:")
		for _, route := range routes {
			lines = append(lines, "  - "+route)
		}
	}
	if rows := rowsSummary(m["rows"]); rows != "" {
		lines = append(lines, "- rows:")
		lines = append(lines, "```yaml")
		lines = append(lines, rows)
		lines = append(lines, "```")
	}

	remaining := copyMap(m)
	delete(remaining, "summaryPath")
	delete(remaining, "previewPath")
	delete(remaining, "note")
	delete(remaining, "routes")
	delete(remaining, "rows")
	if len(remaining) > 0 {
		if y, err := yaml.Marshal(remaining); err == nil {
			lines = append(lines, "- extra:")
			lines = append(lines, "```yaml")
			lines = append(lines, strings.TrimSpace(string(y)))
			lines = append(lines, "```")
		}
	}

	return strings.Join(lines, "\n")
}

func routeLines(v any) []string {
	items, ok := v.([]any)
	if !ok {
		if routes, ok := v.([]map[string]any); ok {
			items = make([]any, 0, len(routes))
			for _, route := range routes {
				items = append(items, route)
			}
		} else {
			return nil
		}
	}
	ret := make([]string, 0, len(items))
	for _, item := range items {
		route, ok := item.(map[string]any)
		if !ok {
			continue
		}
		method, _ := stringField(route, "method")
		path, _ := stringField(route, "path")
		if method == "" && path == "" {
			continue
		}
		ret = append(ret, strings.TrimSpace(method+" "+path))
	}
	sort.Strings(ret)
	return ret
}

func rowsSummary(v any) string {
	items, ok := v.([]any)
	if !ok {
		if rows, ok := v.([]map[string]any); ok {
			items = make([]any, 0, len(rows))
			for _, row := range rows {
				items = append(items, row)
			}
		} else {
			return ""
		}
	}
	if len(items) == 0 {
		return ""
	}
	if len(items) > 5 {
		items = items[:5]
	}
	if y, err := yaml.Marshal(items); err == nil {
		return strings.TrimSpace(string(y))
	}
	return ""
}

func stringField(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(s), true
}

func mapField(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	ret, ok := v.(map[string]any)
	return ret, ok
}

func copyMap(m map[string]any) map[string]any {
	ret := make(map[string]any, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}

func fencedAny(v any) string {
	switch typed := v.(type) {
	case string:
		if parsed := strings.TrimSpace(typed); parsed != "" && (strings.HasPrefix(parsed, "{") || strings.HasPrefix(parsed, "[")) {
			var anyv any
			if json.Unmarshal([]byte(parsed), &anyv) == nil {
				if y, err := yaml.Marshal(anyv); err == nil {
					return "```yaml\n" + strings.TrimSpace(string(y)) + "\n```"
				}
			}
		}
		return "```text\n" + strings.TrimSpace(typed) + "\n```"
	default:
		if y, err := yaml.Marshal(typed); err == nil {
			return "```yaml\n" + strings.TrimSpace(string(y)) + "\n```"
		}
		return fmt.Sprintf("```text\n%v\n```", typed)
	}
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
