package app

import (
	"encoding/json"

	geppettoobs "github.com/go-go-golems/geppetto/pkg/observability"
)

type GeppettoDebugRecord struct {
	Stage       string `json:"stage"`
	Kind        string `json:"kind,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
	EventType   string `json:"eventType,omitempty"`
	InfoMessage string `json:"infoMessage,omitempty"`

	MessageID   string `json:"messageId,omitempty"`
	SessionID   string `json:"sessionId,omitempty"`
	RunID       string `json:"runId,omitempty"`
	InferenceID string `json:"inferenceId,omitempty"`
	TurnID      string `json:"turnId,omitempty"`

	ProviderCallID    string `json:"providerCallId,omitempty"`
	ProviderCallIndex *int   `json:"providerCallIndex,omitempty"`
	ResponseID        string `json:"responseId,omitempty"`
	ItemID            string `json:"itemId,omitempty"`
	OutputIndex       *int   `json:"outputIndex,omitempty"`
	SummaryIndex      *int   `json:"summaryIndex,omitempty"`

	ChoiceIndex          *int   `json:"choiceIndex,omitempty"`
	ContentBlockIndex    *int   `json:"contentBlockIndex,omitempty"`
	StreamKind           string `json:"streamKind,omitempty"`
	CorrelationKey       string `json:"correlationKey,omitempty"`
	ParentCorrelationKey string `json:"parentCorrelationKey,omitempty"`
	ToolCallID           string `json:"toolCallId,omitempty"`
	ToolCallIndex        *int   `json:"toolCallIndex,omitempty"`

	SegmentID     string `json:"segmentId,omitempty"`
	SegmentIndex  *int   `json:"segmentIndex,omitempty"`
	SegmentType   string `json:"segmentType,omitempty"`
	SegmentStatus string `json:"segmentStatus,omitempty"`
	TextLen       int    `json:"textLen,omitempty"`

	StopReason   string `json:"stopReason,omitempty"`
	FinishClass  string `json:"finishClass,omitempty"`
	Usage        any    `json:"usage,omitempty"`
	DurationMs   *int64 `json:"durationMs,omitempty"`
	HasToolCalls bool   `json:"hasToolCalls,omitempty"`

	DeltaLen           int `json:"deltaLen,omitempty"`
	NormalizedDeltaLen int `json:"normalizedDeltaLen,omitempty"`
	BufferLen          int `json:"bufferLen,omitempty"`

	ObjectJSON   any    `json:"objectJson,omitempty"`
	EventJSON    any    `json:"eventJson,omitempty"`
	MetadataJSON any    `json:"metadataJson,omitempty"`
	Error        string `json:"error,omitempty"`
}

func encodeGeppettoRecord(rec geppettoobs.Record) *GeppettoDebugRecord {
	return &GeppettoDebugRecord{
		Stage:                string(rec.Stage),
		Kind:                 string(rec.Kind),
		Provider:             rec.Provider,
		Model:                rec.Model,
		EventType:            rec.EventType,
		InfoMessage:          rec.InfoMessage,
		MessageID:            rec.MessageID,
		SessionID:            rec.SessionID,
		RunID:                rec.RunID,
		InferenceID:          rec.InferenceID,
		TurnID:               rec.TurnID,
		ProviderCallID:       rec.ProviderCallID,
		ProviderCallIndex:    rec.ProviderCallIndex,
		ResponseID:           rec.ResponseID,
		ItemID:               rec.ItemID,
		OutputIndex:          rec.OutputIndex,
		SummaryIndex:         rec.SummaryIndex,
		ChoiceIndex:          rec.ChoiceIndex,
		ContentBlockIndex:    rec.ContentBlockIndex,
		StreamKind:           rec.StreamKind,
		CorrelationKey:       rec.CorrelationKey,
		ParentCorrelationKey: rec.ParentCorrelationKey,
		ToolCallID:           rec.ToolCallID,
		ToolCallIndex:        rec.ToolCallIndex,
		SegmentID:            rec.SegmentID,
		SegmentIndex:         rec.SegmentIndex,
		SegmentType:          rec.SegmentType,
		SegmentStatus:        rec.SegmentStatus,
		TextLen:              rec.TextLen,
		StopReason:           rec.StopReason,
		FinishClass:          rec.FinishClass,
		Usage:                rec.Usage,
		DurationMs:           rec.DurationMs,
		HasToolCalls:         rec.HasToolCalls,
		DeltaLen:             rec.DeltaLen,
		NormalizedDeltaLen:   rec.NormalizedDeltaLen,
		BufferLen:            rec.BufferLen,
		ObjectJSON:           decodeJSONRaw(rec.ObjectJSON),
		EventJSON:            decodeJSONRaw(rec.EventJSON),
		MetadataJSON:         decodeJSONRaw(rec.MetadataJSON),
		Error:                rec.Error,
	}
}

func decodeJSONRaw(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return string(raw)
	}
	return out
}
