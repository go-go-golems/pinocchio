package export

import (
	"errors"
	"strings"
)

type Format string

const (
	FormatJSON      Format = "json"
	FormatYAML      Format = "yaml"
	FormatMarkdown  Format = "markdown"
	FormatMinitrace Format = "minitrace"
)

type TimelineView string

const (
	TimelineViewMessages TimelineView = "messages"
	TimelineViewEntities TimelineView = "entities"
	TimelineViewTurns    TimelineView = "turns"
)

var (
	ErrInvalidFormat        = errors.New("invalid export format")
	ErrInvalidView          = errors.New("invalid timeline export view")
	ErrSnapshotUnavailable  = errors.New("snapshot provider unavailable")
	ErrTurnStoreUnavailable = errors.New("turn store unavailable")
	ErrTurnsDBPathRequired  = errors.New("minitrace export requires a file-backed turns database")
	ErrNotFound             = errors.New("export source not found")
)

type Options struct {
	Format          Format
	View            TimelineView
	Download        bool
	TurnPhase       string
	Limit           int
	LatestOnly      bool
	IncludeTimeline bool
	IncludeTurns    bool
}

func (o Options) Normalized() (Options, error) {
	out := o
	out.Format = NormalizeFormat(out.Format)
	if out.Format == "" {
		out.Format = FormatJSON
	}
	if !out.Format.Valid() {
		return out, ErrInvalidFormat
	}
	out.View = NormalizeTimelineView(out.View)
	if out.View == "" {
		out.View = TimelineViewEntities
	}
	if !out.View.Valid() {
		return out, ErrInvalidView
	}
	out.TurnPhase = strings.TrimSpace(out.TurnPhase)
	if out.TurnPhase == "" {
		out.TurnPhase = "final"
	}
	if out.Limit <= 0 {
		out.Limit = 1000
	}
	return out, nil
}

func NormalizeFormat(format Format) Format {
	return Format(strings.ToLower(strings.TrimSpace(string(format))))
}

func (f Format) Valid() bool {
	switch NormalizeFormat(f) {
	case FormatJSON, FormatYAML, FormatMarkdown, FormatMinitrace:
		return true
	default:
		return false
	}
}

func NormalizeTimelineView(view TimelineView) TimelineView {
	return TimelineView(strings.ToLower(strings.TrimSpace(string(view))))
}

func (v TimelineView) Valid() bool {
	switch NormalizeTimelineView(v) {
	case TimelineViewMessages, TimelineViewEntities, TimelineViewTurns:
		return true
	default:
		return false
	}
}

type TimelineExport struct {
	SessionID       string         `json:"session_id" yaml:"session_id"`
	SnapshotOrdinal uint64         `json:"snapshot_ordinal" yaml:"snapshot_ordinal"`
	View            TimelineView   `json:"view" yaml:"view"`
	ExportedAt      string         `json:"exported_at" yaml:"exported_at"`
	Entities        []EntityExport `json:"entities,omitempty" yaml:"entities,omitempty"`
}

type EntityExport struct {
	Kind             string `json:"kind" yaml:"kind"`
	ID               string `json:"id" yaml:"id"`
	CreatedOrdinal   uint64 `json:"created_ordinal" yaml:"created_ordinal"`
	LastEventOrdinal uint64 `json:"last_event_ordinal" yaml:"last_event_ordinal"`
	Tombstone        bool   `json:"tombstone" yaml:"tombstone"`
	Payload          any    `json:"payload,omitempty" yaml:"payload,omitempty"`
}

type TurnsExport struct {
	SessionID  string               `json:"session_id" yaml:"session_id"`
	Phase      string               `json:"phase" yaml:"phase"`
	ExportedAt string               `json:"exported_at" yaml:"exported_at"`
	Turns      []TurnSnapshotExport `json:"turns" yaml:"turns"`
}

type TurnSnapshotExport struct {
	ConvID      string `json:"conv_id" yaml:"conv_id"`
	SessionID   string `json:"session_id" yaml:"session_id"`
	TurnID      string `json:"turn_id" yaml:"turn_id"`
	Phase       string `json:"phase" yaml:"phase"`
	RuntimeKey  string `json:"runtime_key,omitempty" yaml:"runtime_key,omitempty"`
	InferenceID string `json:"inference_id,omitempty" yaml:"inference_id,omitempty"`
	CreatedAtMs int64  `json:"created_at_ms" yaml:"created_at_ms"`
	CreatedAt   string `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	Payload     string `json:"payload" yaml:"payload"`
}
