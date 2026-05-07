package export

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type SnapshotProvider interface {
	Snapshot(ctx context.Context, sid sessionstream.SessionId) (sessionstream.Snapshot, error)
}

type Service struct {
	snapshotProvider SnapshotProvider
	turnStore        chatstore.TurnStore
	turnsDBPath      string
	now              func() time.Time
}

type Option func(*Service)

func NewService(snapshotProvider SnapshotProvider, opts ...Option) *Service {
	s := &Service{snapshotProvider: snapshotProvider, now: time.Now}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

func WithTurnStore(store chatstore.TurnStore) Option {
	return func(s *Service) {
		if s != nil {
			s.turnStore = store
		}
	}
}

func WithTurnsDBPath(path string) Option {
	return func(s *Service) {
		if s != nil {
			s.turnsDBPath = strings.TrimSpace(path)
		}
	}
}

func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if s != nil && now != nil {
			s.now = now
		}
	}
}

func (s *Service) ExportTimeline(ctx context.Context, sessionID string, opts Options) (*TimelineExport, error) {
	if s == nil || s.snapshotProvider == nil {
		return nil, ErrSnapshotUnavailable
	}
	normalized, err := opts.Normalized()
	if err != nil {
		return nil, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.Wrap(ErrNotFound, "session id is empty")
	}

	snap, err := s.snapshotProvider.Snapshot(ctx, sessionstream.SessionId(sessionID))
	if err != nil {
		return nil, err
	}
	out := &TimelineExport{
		SessionID:       string(snap.SessionId),
		SnapshotOrdinal: snap.SnapshotOrdinal,
		View:            normalized.View,
		ExportedAt:      s.formatNow(),
		Entities:        make([]EntityExport, 0, len(snap.Entities)),
	}
	for _, entity := range snap.Entities {
		payload, err := protoToExportValue(entity.Payload)
		if err != nil {
			payload = map[string]any{"error": err.Error()}
		}
		out.Entities = append(out.Entities, EntityExport{
			Kind:             entity.Kind,
			ID:               entity.Id,
			CreatedOrdinal:   entity.CreatedOrdinal,
			LastEventOrdinal: entity.LastEventOrdinal,
			Tombstone:        entity.Tombstone,
			Payload:          payload,
		})
	}
	return out, nil
}

func (s *Service) ExportTurns(ctx context.Context, sessionID string, opts Options) (*TurnsExport, error) {
	if s == nil || s.turnStore == nil {
		return nil, ErrTurnStoreUnavailable
	}
	normalized, err := opts.Normalized()
	if err != nil {
		return nil, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.Wrap(ErrNotFound, "session id is empty")
	}
	turns, err := s.turnStore.List(ctx, chatstore.TurnQuery{
		ConvID: sessionID,
		Phase:  normalized.TurnPhase,
		Limit:  normalized.Limit,
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(turns, func(i, j int) bool {
		if turns[i].CreatedAtMs != turns[j].CreatedAtMs {
			return turns[i].CreatedAtMs < turns[j].CreatedAtMs
		}
		return turns[i].TurnID < turns[j].TurnID
	})
	if normalized.LatestOnly && len(turns) > 0 {
		turns = turns[len(turns)-1:]
	}
	out := &TurnsExport{
		SessionID:  sessionID,
		Phase:      normalized.TurnPhase,
		ExportedAt: s.formatNow(),
		Turns:      make([]TurnSnapshotExport, 0, len(turns)),
	}
	for _, turn := range turns {
		out.Turns = append(out.Turns, TurnSnapshotExport{
			ConvID:      turn.ConvID,
			SessionID:   turn.SessionID,
			TurnID:      turn.TurnID,
			Phase:       turn.Phase,
			RuntimeKey:  turn.RuntimeKey,
			InferenceID: turn.InferenceID,
			CreatedAtMs: turn.CreatedAtMs,
			CreatedAt:   formatMillis(turn.CreatedAtMs),
			Payload:     turn.Payload,
		})
	}
	return out, nil
}

func (s *Service) formatNow() string {
	now := time.Now
	if s != nil && s.now != nil {
		now = s.now
	}
	return now().UTC().Format(time.RFC3339)
}

func formatMillis(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func protoToExportValue(msg proto.Message) (any, error) {
	if msg == nil {
		return nil, nil
	}
	body, err := protojson.MarshalOptions{EmitUnpopulated: false, UseProtoNames: true}.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
