package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	wstransport "github.com/go-go-golems/sessionstream/pkg/sessionstream/transport/ws"
	"google.golang.org/protobuf/proto"
)

const defaultDebugRecorderMaxRecords = 10000

type DebugRecordKind string

const (
	DebugRecordKindPipeline  DebugRecordKind = "pipeline"
	DebugRecordKindTransport DebugRecordKind = "transport"
)

type DebugRecord struct {
	Kind      DebugRecordKind `json:"kind"`
	Timestamp time.Time       `json:"timestamp"`

	SessionID    string `json:"sessionId,omitempty"`
	ConnectionID string `json:"connectionId,omitempty"`
	Ordinal      string `json:"ordinal,omitempty"`

	Pipeline  *PipelineDebugRecord  `json:"pipeline,omitempty"`
	Transport *TransportDebugRecord `json:"transport,omitempty"`
}

type PipelineDebugRecord struct {
	Mode     string `json:"mode"`
	Event    string `json:"event"`
	EventTyp string `json:"eventType,omitempty"`
	Payload  any    `json:"payload,omitempty"`

	EventAppended bool   `json:"eventAppended"`
	AppendError   string `json:"appendError,omitempty"`
	SessionError  string `json:"sessionError,omitempty"`

	ViewOrdinal string `json:"viewOrdinal,omitempty"`
	ViewError   string `json:"viewError,omitempty"`

	UIProjectionError       string                `json:"uiProjectionError,omitempty"`
	TimelineProjectionError string                `json:"timelineProjectionError,omitempty"`
	ApplyError              string                `json:"applyError,omitempty"`
	CursorError             string                `json:"cursorError,omitempty"`
	FanoutError             string                `json:"fanoutError,omitempty"`
	TimelineCursorAdvanced  bool                  `json:"timelineCursorAdvanced"`
	UIEvents                []UIEventDebug        `json:"uiEvents,omitempty"`
	TimelineEntities        []TimelineEntityDebug `json:"timelineEntities,omitempty"`
	AppliedEntities         []TimelineEntityDebug `json:"appliedEntities,omitempty"`
	FanoutEvents            []UIEventDebug        `json:"fanoutEvents,omitempty"`
}

type UIEventDebug struct {
	Name        string `json:"name"`
	PayloadType string `json:"payloadType,omitempty"`
	Payload     any    `json:"payload,omitempty"`
}

type TimelineEntityDebug struct {
	Kind             string `json:"kind"`
	ID               string `json:"id"`
	CreatedOrdinal   string `json:"createdOrdinal,omitempty"`
	LastEventOrdinal string `json:"lastEventOrdinal,omitempty"`
	Tombstone        bool   `json:"tombstone,omitempty"`
	PayloadType      string `json:"payloadType,omitempty"`
	Payload          any    `json:"payload,omitempty"`
}

type TransportDebugRecord struct {
	Stage       string `json:"stage"`
	Direction   string `json:"direction,omitempty"`
	FrameType   string `json:"frameType,omitempty"`
	EventName   string `json:"eventName,omitempty"`
	PayloadType string `json:"payloadType,omitempty"`

	SinceSnapshotOrdinal string                        `json:"sinceSnapshotOrdinal,omitempty"`
	SnapshotOrdinal      string                        `json:"snapshotOrdinal,omitempty"`
	SnapshotEntityCount  int                           `json:"snapshotEntityCount,omitempty"`
	SnapshotEntities     []TransportEntitySummaryDebug `json:"snapshotEntities,omitempty"`

	FanoutEventCount int      `json:"fanoutEventCount,omitempty"`
	FanoutTargetIDs  []string `json:"fanoutTargetIds,omitempty"`

	RawBytes int    `json:"rawBytes,omitempty"`
	QueueLen int    `json:"queueLen,omitempty"`
	QueueCap int    `json:"queueCap,omitempty"`
	Error    string `json:"error,omitempty"`
}

type TransportEntitySummaryDebug struct {
	Kind             string `json:"kind"`
	ID               string `json:"id"`
	CreatedOrdinal   string `json:"createdOrdinal,omitempty"`
	LastEventOrdinal string `json:"lastEventOrdinal,omitempty"`
	PayloadType      string `json:"payloadType,omitempty"`
	Tombstone        bool   `json:"tombstone,omitempty"`
}

type StreamDebugRecorder struct {
	mu         sync.RWMutex
	maxRecords int
	records    []DebugRecord
}

func NewStreamDebugRecorder(maxRecords int) *StreamDebugRecorder {
	if maxRecords <= 0 {
		maxRecords = defaultDebugRecorderMaxRecords
	}
	return &StreamDebugRecorder{maxRecords: maxRecords}
}

func (r *StreamDebugRecorder) OnPipeline(ctx context.Context, rec sessionstream.PipelineRecord) {
	r.RecordPipeline(ctx, rec)
}

func (r *StreamDebugRecorder) OnTransport(ctx context.Context, rec wstransport.TransportRecord) {
	r.RecordTransport(ctx, rec)
}

func (r *StreamDebugRecorder) RecordPipeline(_ context.Context, rec sessionstream.PipelineRecord) {
	if r == nil {
		return
	}
	out := DebugRecord{
		Kind:      DebugRecordKindPipeline,
		Timestamp: time.Now().UTC(),
		SessionID: string(rec.SessionId),
		Ordinal:   formatUint(rec.Ordinal),
		Pipeline:  encodePipelineRecord(rec),
	}
	r.append(out)
}

func (r *StreamDebugRecorder) RecordTransport(_ context.Context, rec wstransport.TransportRecord) {
	if r == nil {
		return
	}
	out := DebugRecord{
		Kind:         DebugRecordKindTransport,
		Timestamp:    time.Now().UTC(),
		SessionID:    string(rec.SessionId),
		ConnectionID: string(rec.ConnectionId),
		Ordinal:      formatUint(rec.Ordinal),
		Transport:    encodeTransportRecord(rec),
	}
	r.append(out)
}

func (r *StreamDebugRecorder) Records(sessionID string, kind DebugRecordKind) []DebugRecord {
	if r == nil {
		return nil
	}
	sessionID = strings.TrimSpace(sessionID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]DebugRecord, 0, len(r.records))
	for _, rec := range r.records {
		if sessionID != "" && rec.SessionID != sessionID {
			continue
		}
		if kind != "" && rec.Kind != kind {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *StreamDebugRecorder) append(rec DebugRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
	if len(r.records) > r.maxRecords {
		copy(r.records, r.records[len(r.records)-r.maxRecords:])
		r.records = r.records[:r.maxRecords]
	}
}

func encodePipelineRecord(rec sessionstream.PipelineRecord) *PipelineDebugRecord {
	return &PipelineDebugRecord{
		Mode:                    string(rec.Mode),
		Event:                   rec.EventName,
		EventTyp:                protoType(rec.Event.Payload),
		Payload:                 encodeProtoJSON(rec.Event.Payload),
		EventAppended:           rec.EventAppended,
		AppendError:             errString(rec.AppendErr),
		SessionError:            errString(rec.SessionErr),
		ViewOrdinal:             formatUint(rec.ViewOrdinal),
		ViewError:               errString(rec.ViewErr),
		UIProjectionError:       errString(rec.UIProjectionErr),
		TimelineProjectionError: errString(rec.TimelineProjectionErr),
		ApplyError:              errString(rec.ApplyErr),
		CursorError:             errString(rec.CursorErr),
		FanoutError:             errString(rec.FanoutErr),
		TimelineCursorAdvanced:  rec.TimelineCursorAdvanced,
		UIEvents:                encodeUIEvents(rec.UIEvents),
		TimelineEntities:        encodeTimelineEntities(rec.TimelineEntities),
		AppliedEntities:         encodeTimelineEntities(rec.AppliedEntities),
		FanoutEvents:            encodeUIEvents(rec.FanoutEvents),
	}
}

func encodeTransportRecord(rec wstransport.TransportRecord) *TransportDebugRecord {
	return &TransportDebugRecord{
		Stage:                string(rec.Stage),
		Direction:            string(rec.Direction),
		FrameType:            rec.FrameType,
		EventName:            rec.EventName,
		PayloadType:          rec.PayloadType,
		SinceSnapshotOrdinal: formatUint(rec.SinceSnapshotOrdinal),
		SnapshotOrdinal:      formatUint(rec.SnapshotOrdinal),
		SnapshotEntityCount:  rec.SnapshotEntityCount,
		SnapshotEntities:     encodeTransportEntities(rec.SnapshotEntities),
		FanoutEventCount:     rec.FanoutEventCount,
		FanoutTargetIDs:      encodeConnectionIDs(rec.FanoutTargetIds),
		RawBytes:             rec.RawBytes,
		QueueLen:             rec.QueueLen,
		QueueCap:             rec.QueueCap,
		Error:                errString(rec.Err),
	}
}

func encodeUIEvents(events []sessionstream.UIEvent) []UIEventDebug {
	if len(events) == 0 {
		return nil
	}
	out := make([]UIEventDebug, 0, len(events))
	for _, ev := range events {
		out = append(out, UIEventDebug{Name: ev.Name, PayloadType: protoType(ev.Payload), Payload: encodeProtoJSON(ev.Payload)})
	}
	return out
}

func encodeTimelineEntities(entities []sessionstream.TimelineEntity) []TimelineEntityDebug {
	if len(entities) == 0 {
		return nil
	}
	out := make([]TimelineEntityDebug, 0, len(entities))
	for _, entity := range entities {
		out = append(out, TimelineEntityDebug{
			Kind:             entity.Kind,
			ID:               entity.Id,
			CreatedOrdinal:   formatUint(entity.CreatedOrdinal),
			LastEventOrdinal: formatUint(entity.LastEventOrdinal),
			Tombstone:        entity.Tombstone,
			PayloadType:      protoType(entity.Payload),
			Payload:          encodeProtoJSON(entity.Payload),
		})
	}
	return out
}

func encodeTransportEntities(entities []wstransport.TimelineEntitySummary) []TransportEntitySummaryDebug {
	if len(entities) == 0 {
		return nil
	}
	out := make([]TransportEntitySummaryDebug, 0, len(entities))
	for _, entity := range entities {
		out = append(out, TransportEntitySummaryDebug{
			Kind:             entity.Kind,
			ID:               entity.Id,
			CreatedOrdinal:   formatUint(entity.CreatedOrdinal),
			LastEventOrdinal: formatUint(entity.LastEventOrdinal),
			PayloadType:      entity.PayloadType,
			Tombstone:        entity.Tombstone,
		})
	}
	return out
}

func encodeConnectionIDs(ids []sessionstream.ConnectionId) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, string(id))
	}
	return out
}

func protoType(msg proto.Message) string {
	if msg == nil {
		return ""
	}
	return string(msg.ProtoReflect().Descriptor().FullName())
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func formatUint(v uint64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%d", v)
}
