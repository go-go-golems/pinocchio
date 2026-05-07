package app

import sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"

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
