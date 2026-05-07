package app

import (
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	wstransport "github.com/go-go-golems/sessionstream/pkg/sessionstream/transport/ws"
)

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
