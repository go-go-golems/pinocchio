package agentmode

import gepevents "github.com/go-go-golems/geppetto/pkg/events"

const EventTypeModeSwitchPreview gepevents.EventType = "agent-mode-preview"

// EventModeSwitchPreview is a transient streaming event emitted by the structured
// sink while the model is still producing a mode-switch payload. It is useful for
// progressive UI previews but should not be treated as authoritative durable state.
type EventModeSwitchPreview struct {
	gepevents.EventImpl
	ItemID        string `json:"item_id,omitempty"`
	CandidateMode string `json:"candidate_mode,omitempty"`
	Analysis      string `json:"analysis,omitempty"`
	ParseState    string `json:"parse_state,omitempty"`
}

func NewModeSwitchPreviewEvent(metadata gepevents.EventMetadata, itemID, candidateMode, analysis, parseState string) *EventModeSwitchPreview {
	return &EventModeSwitchPreview{
		EventImpl:     gepevents.EventImpl{Type_: EventTypeModeSwitchPreview, Metadata_: metadata},
		ItemID:        itemID,
		CandidateMode: candidateMode,
		Analysis:      analysis,
		ParseState:    parseState,
	}
}

var _ gepevents.Event = (*EventModeSwitchPreview)(nil)
