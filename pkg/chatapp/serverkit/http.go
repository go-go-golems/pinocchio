package serverkit

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// WriteJSON writes payload as a JSON response.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

const maxInt64AsUint64 = uint64(1<<63 - 1)

func uint64ToInt64OrZero(v uint64) int64 {
	if v > maxInt64AsUint64 {
		return 0
	}
	return int64(v)
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
			CreatedAt:        uint64ToInt64OrZero(entity.CreatedOrdinal),
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
