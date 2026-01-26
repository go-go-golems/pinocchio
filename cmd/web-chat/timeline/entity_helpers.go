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
	var entity timelinepb.TimelineEntityV1
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(trimmed), &entity); err != nil {
		return "", err
	}
	kind := strings.TrimSpace(entity.GetKind())
	if msg := entity.GetMessage(); msg != nil {
		role := strings.TrimSpace(msg.GetRole())
		content := truncateString(msg.GetContent(), 160)
		if role == "" {
			role = "message"
		}
		if content != "" {
			return fmt.Sprintf("%s: %s", role, content), nil
		}
		return role, nil
	}
	if tc := entity.GetToolCall(); tc != nil {
		name := strings.TrimSpace(tc.GetName())
		status := strings.TrimSpace(tc.GetStatus())
		if name == "" {
			name = "tool_call"
		}
		if status != "" {
			return fmt.Sprintf("tool_call %s (%s)", name, status), nil
		}
		return fmt.Sprintf("tool_call %s", name), nil
	}
	if tr := entity.GetToolResult(); tr != nil {
		toolID := strings.TrimSpace(tr.GetToolCallId())
		errText := truncateString(tr.GetError(), 120)
		if toolID == "" {
			toolID = "tool_result"
		}
		if errText != "" {
			return fmt.Sprintf("tool_result %s error=%s", toolID, errText), nil
		}
		return fmt.Sprintf("tool_result %s", toolID), nil
	}
	if st := entity.GetStatus(); st != nil {
		text := truncateString(st.GetText(), 160)
		if text != "" {
			if st.GetType() != "" {
				return fmt.Sprintf("status %s: %s", st.GetType(), text), nil
			}
			return fmt.Sprintf("status: %s", text), nil
		}
		if st.GetType() != "" {
			return fmt.Sprintf("status %s", st.GetType()), nil
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
