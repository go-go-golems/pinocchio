package app

import (
	"encoding/json"

	geppettoobs "github.com/go-go-golems/geppetto/pkg/observability"
)

type GeppettoDebugRecord struct {
	Stage       string `json:"stage"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
	EventType   string `json:"eventType,omitempty"`
	InfoMessage string `json:"infoMessage,omitempty"`

	MessageID   string `json:"messageId,omitempty"`
	InferenceID string `json:"inferenceId,omitempty"`
	TurnID      string `json:"turnId,omitempty"`

	ResponseID   string `json:"responseId,omitempty"`
	ItemID       string `json:"itemId,omitempty"`
	OutputIndex  *int   `json:"outputIndex,omitempty"`
	SummaryIndex *int   `json:"summaryIndex,omitempty"`

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
		Stage:              string(rec.Stage),
		Provider:           rec.Provider,
		Model:              rec.Model,
		EventType:          rec.EventType,
		InfoMessage:        rec.InfoMessage,
		MessageID:          rec.MessageID,
		InferenceID:        rec.InferenceID,
		TurnID:             rec.TurnID,
		ResponseID:         rec.ResponseID,
		ItemID:             rec.ItemID,
		OutputIndex:        rec.OutputIndex,
		SummaryIndex:       rec.SummaryIndex,
		DeltaLen:           rec.DeltaLen,
		NormalizedDeltaLen: rec.NormalizedDeltaLen,
		BufferLen:          rec.BufferLen,
		ObjectJSON:         decodeJSONRaw(rec.ObjectJSON),
		EventJSON:          decodeJSONRaw(rec.EventJSON),
		MetadataJSON:       decodeJSONRaw(rec.MetadataJSON),
		Error:              rec.Error,
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
