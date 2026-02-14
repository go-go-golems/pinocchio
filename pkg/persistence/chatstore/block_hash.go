package chatstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// BlockContentHashAlgorithmV1 identifies the canonical hash material/version.
//
// The canonical material is JSON over:
//   - kind
//   - role
//   - payload
//   - metadata
//
// with payload/metadata nil treated as empty object.
const BlockContentHashAlgorithmV1 = "sha256-canonical-json-v1"

type canonicalBlockMaterial struct {
	Kind     string `json:"kind"`
	Role     string `json:"role"`
	Payload  any    `json:"payload"`
	Metadata any    `json:"metadata"`
}

// CanonicalBlockMaterialJSON returns the canonical JSON bytes used for block hashing.
func CanonicalBlockMaterialJSON(kind, role string, payload, metadata map[string]any) ([]byte, error) {
	m := canonicalBlockMaterial{
		Kind:     strings.TrimSpace(kind),
		Role:     strings.TrimSpace(role),
		Payload:  normalizeJSONValue(payload),
		Metadata: normalizeJSONValue(metadata),
	}
	if m.Payload == nil {
		m.Payload = map[string]any{}
	}
	if m.Metadata == nil {
		m.Metadata = map[string]any{}
	}
	return json.Marshal(m)
}

// ComputeBlockContentHash computes the lowercase-hex SHA-256 hash over canonical block material.
func ComputeBlockContentHash(kind, role string, payload, metadata map[string]any) (string, error) {
	b, err := CanonicalBlockMaterialJSON(kind, role, payload, metadata)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func normalizeJSONValue(v any) any {
	if v == nil {
		return nil
	}

	switch vv := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(vv))
		for k, value := range vv {
			out[k] = normalizeJSONValue(value)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(vv))
		for k, value := range vv {
			out[fmt.Sprint(k)] = normalizeJSONValue(value)
		}
		return out
	case []any:
		out := make([]any, len(vv))
		for i := range vv {
			out[i] = normalizeJSONValue(vv[i])
		}
		return out
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Map {
		out := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			out[fmt.Sprint(iter.Key().Interface())] = normalizeJSONValue(iter.Value().Interface())
		}
		return out
	}
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = normalizeJSONValue(rv.Index(i).Interface())
		}
		return out
	}
	return v
}
