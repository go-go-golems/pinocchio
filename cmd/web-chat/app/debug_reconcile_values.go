package app

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func mustJSON(v any) string {
	if v == nil {
		return "null"
	}
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(body)
}

func nullableIntPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt(s string) any {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return v
}

func nullableIntFromAny(v any) any {
	i := int64FromAny(v)
	if i == 0 {
		return nil
	}
	return i
}

func int64FromAny(v any) int64 {
	switch typed := v.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		out, _ := typed.Int64()
		return out
	case string:
		out, _ := strconv.ParseInt(typed, 10, 64)
		return out
	default:
		return 0
	}
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func boolFromAny(v any) bool {
	b, _ := v.(bool)
	return b
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func arrayFromAny(v any) []any {
	items, _ := v.([]any)
	return items
}
