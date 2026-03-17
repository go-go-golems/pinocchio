package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/go-go-golems/bobatea/pkg/timeline"
	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
	"github.com/go-go-golems/pinocchio/cmd/examples/internal/demorender"
	"gopkg.in/yaml.v3"
)

func registerDemoRenderers(r *timeline.Registry) {
	demorender.RegisterBaseRenderers(r,
		demorender.NewToolCallFactory("renderer.tool_call.scopedjs_eval.v1", demoToolName, buildEvalToolCallMarkdown),
		demorender.NewResultFactory("renderer.tool_call_result.scopedjs_eval.v1", formatEvalResultMarkdownWithWidth),
	)
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
	return demorender.FencedAny(inputRaw)
}

func formatEvalResultMarkdown(raw string) string {
	return formatEvalResultMarkdownWithWidth(raw, 0)
}

func formatEvalResultMarkdownWithWidth(raw string, _ int) string {
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
		return "result:\n" + demorender.FencedAny(v)
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
