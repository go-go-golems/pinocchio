package app

import (
	"context"

	chatexport "github.com/go-go-golems/pinocchio/pkg/chatapp/export"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// DebugTimelineProvider fetches timeline snapshot data for the reconcile DB.
type DebugTimelineProvider interface {
	ExportTimelineEntities(ctx context.Context, sessionID string) ([]DebugTimelineEntity, error)
}

// DebugTurnsProvider fetches turns data for the reconcile DB.
type DebugTurnsProvider interface {
	ExportTurnsList(ctx context.Context, sessionID string) ([]DebugTurn, error)
}

// DebugTimelineEntity is a flat timeline entity row for the reconcile DB.
type DebugTimelineEntity struct {
	Kind             string `json:"kind"`
	ID               string `json:"id"`
	CreatedOrdinal   uint64 `json:"createdOrdinal"`
	LastEventOrdinal uint64 `json:"lastEventOrdinal"`
	Tombstone        bool   `json:"tombstone"`
	PayloadType      string `json:"payloadType,omitempty"`
	Payload          string `json:"payload,omitempty"`
}

// DebugTurn is a flat turn row for the reconcile DB.
type DebugTurn struct {
	ConvID      string `json:"convId"`
	SessionID   string `json:"sessionId"`
	TurnID      string `json:"turnId"`
	Phase       string `json:"phase"`
	RuntimeKey  string `json:"runtimeKey,omitempty"`
	InferenceID string `json:"inferenceId,omitempty"`
	CreatedAtMs int64  `json:"createdAtMs"`
	CreatedAt   string `json:"createdAt,omitempty"`
	Payload     string `json:"payload"`
}

// DebugDataProvider combines timeline and turns providers.
type DebugDataProvider interface {
	DebugTimelineProvider
	DebugTurnsProvider
}

type exportDataProvider struct {
	snapshotProvider chatexport.SnapshotProvider
	turnStore        chatstore.TurnStore
}

func newExportDataProvider(snapshotProvider chatexport.SnapshotProvider, turnStore chatstore.TurnStore) *exportDataProvider {
	if snapshotProvider == nil && turnStore == nil {
		return nil
	}
	return &exportDataProvider{snapshotProvider: snapshotProvider, turnStore: turnStore}
}

func (p *exportDataProvider) ExportTimelineEntities(ctx context.Context, sessionID string) ([]DebugTimelineEntity, error) {
	if p == nil || p.snapshotProvider == nil {
		return nil, nil
	}
	snap, err := p.snapshotProvider.Snapshot(ctx, sessionstream.SessionId(sessionID))
	if err != nil {
		return nil, err
	}
	entities := make([]DebugTimelineEntity, 0, len(snap.Entities))
	for _, ent := range snap.Entities {
		entities = append(entities, DebugTimelineEntity{
			Kind:             ent.Kind,
			ID:               ent.Id,
			CreatedOrdinal:   ent.CreatedOrdinal,
			LastEventOrdinal: ent.LastEventOrdinal,
			Tombstone:        ent.Tombstone,
			PayloadType:      protoType(ent.Payload),
			Payload:          mustJSON(encodeProtoJSON(ent.Payload)),
		})
	}
	return entities, nil
}

func (p *exportDataProvider) ExportTurnsList(ctx context.Context, sessionID string) ([]DebugTurn, error) {
	if p == nil || p.turnStore == nil {
		return nil, nil
	}
	turns, err := p.turnStore.List(ctx, chatstore.TurnQuery{ConvID: sessionID})
	if err != nil {
		return nil, err
	}
	out := make([]DebugTurn, 0, len(turns))
	for _, turn := range turns {
		out = append(out, DebugTurn{
			ConvID:      turn.ConvID,
			SessionID:   turn.SessionID,
			TurnID:      turn.TurnID,
			Phase:       turn.Phase,
			RuntimeKey:  turn.RuntimeKey,
			InferenceID: turn.InferenceID,
			CreatedAtMs: turn.CreatedAtMs,
			Payload:     turn.Payload,
		})
	}
	return out, nil
}
