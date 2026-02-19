package timeline

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

func orderClause(order string) (string, error) {
	clean := strings.TrimSpace(order)
	if clean == "" {
		clean = "version"
	}
	desc := false
	if strings.HasPrefix(clean, "-") {
		desc = true
		clean = strings.TrimPrefix(clean, "-")
	}
	clean = strings.ToLower(strings.TrimSpace(clean))
	cols := map[string]string{
		"version": "version",
		"created": "created_at_ms",
		"updated": "updated_at_ms",
	}
	col, ok := cols[clean]
	if !ok {
		return "", errors.Errorf("invalid order %q (use version|created|updated)", clean)
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	return fmt.Sprintf("%s %s", col, dir), nil
}

func summarizeEntity(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	var entity timelinepb.TimelineEntityV2
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(trimmed), &entity); err != nil {
		return "", err
	}
	kind := strings.TrimSpace(entity.GetKind())
	props := map[string]any{}
	if entity.GetProps() != nil {
		props = entity.GetProps().AsMap()
	}
	if kind == "message" {
		role := strings.TrimSpace(asString(props["role"]))
		content := truncateString(asString(props["content"]), 160)
		if role == "" {
			role = "message"
		}
		if content != "" {
			return fmt.Sprintf("%s: %s", role, content), nil
		}
		return role, nil
	}
	if kind == "tool_call" {
		name := strings.TrimSpace(asString(props["name"]))
		status := strings.TrimSpace(asString(props["status"]))
		if status == "" && asBool(props["done"]) {
			status = "completed"
		}
		if name == "" {
			name = "tool_call"
		}
		if status != "" {
			return fmt.Sprintf("tool_call %s (%s)", name, status), nil
		}
		return fmt.Sprintf("tool_call %s", name), nil
	}
	if kind == "tool_result" {
		toolID := strings.TrimSpace(asString(props["toolCallId"]))
		if toolID == "" {
			toolID = strings.TrimSpace(asString(props["tool_call_id"]))
		}
		errText := truncateString(asString(props["error"]), 120)
		if toolID == "" {
			toolID = "tool_result"
		}
		if errText != "" {
			return fmt.Sprintf("tool_result %s error=%s", toolID, errText), nil
		}
		return fmt.Sprintf("tool_result %s", toolID), nil
	}
	if kind == "status" {
		text := truncateString(asString(props["text"]), 160)
		statusType := strings.TrimSpace(asString(props["type"]))
		if text != "" {
			if statusType != "" {
				return fmt.Sprintf("status %s: %s", statusType, text), nil
			}
			return fmt.Sprintf("status: %s", text), nil
		}
		if statusType != "" {
			return fmt.Sprintf("status %s", statusType), nil
		}
		return "status", nil
	}
	if kind != "" {
		return kind, nil
	}
	return "entity", nil
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if maxLen <= 0 || len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return ""
	}
}

func asBool(v any) bool {
	b, ok := v.(bool)
	return ok && b
}
