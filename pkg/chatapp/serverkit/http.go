package serverkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// DecodeJSON decodes a JSON request body and treats an empty body as valid.
// Callers remain responsible for validating required fields after decoding.
func DecodeJSON(r *http.Request, v any) error {
	if r == nil || r.Body == nil {
		return nil
	}
	defer func() { _ = r.Body.Close() }()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

func ParseSessionPath(path string) (string, string, bool) {
	return ParseSessionPathWithPrefix(path, "/api/chat/sessions/")
}

func ParseSessionPathWithPrefix(path string, prefix string) (string, string, bool) {
	if prefix == "" {
		prefix = "/api/chat/sessions/"
	}
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if rest == "" {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	switch len(parts) {
	case 1:
		return parts[0], "", true
	case 2:
		return parts[0], parts[1], true
	default:
		return "", "", false
	}
}

func EncodeSnapshotResponse(snap sessionstream.Snapshot, statusFn func([]SnapshotEntity) string) SessionSnapshotResponse {
	resp := SessionSnapshotResponse{
		SessionID:       string(snap.SessionId),
		SnapshotOrdinal: fmt.Sprintf("%d", snap.SnapshotOrdinal),
		Entities:        make([]SnapshotEntity, 0, len(snap.Entities)),
	}
	for _, entity := range snap.Entities {
		resp.Entities = append(resp.Entities, SnapshotEntity{
			Kind:             entity.Kind,
			ID:               entity.Id,
			CreatedOrdinal:   fmt.Sprintf("%d", entity.CreatedOrdinal),
			LastEventOrdinal: fmt.Sprintf("%d", entity.LastEventOrdinal),
			Tombstone:        entity.Tombstone,
			Payload:          EncodeProtoJSON(entity.Payload),
			CreatedAt:        int64(entity.CreatedOrdinal),
		})
	}
	if statusFn != nil {
		resp.Status = statusFn(resp.Entities)
	}
	return resp
}

func EncodeProtoJSON(msg proto.Message) any {
	if msg == nil {
		return nil
	}
	body, err := protojson.MarshalOptions{EmitUnpopulated: false, UseProtoNames: false}.Marshal(msg)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return string(body)
	}
	return out
}
