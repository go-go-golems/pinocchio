package serverkit

// SnapshotEntity is one timeline entity in a JSON snapshot.
type SnapshotEntity struct {
	Kind             string `json:"kind"`
	ID               string `json:"id"`
	CreatedOrdinal   string `json:"createdOrdinal,omitempty"`
	LastEventOrdinal string `json:"lastEventOrdinal,omitempty"`
	Tombstone        bool   `json:"tombstone"`
	Payload          any    `json:"payload,omitempty"`
	CreatedAt        int64  `json:"createdAt,omitempty"`
}

// SessionSnapshotResponse is a session's timeline snapshot as JSON.
type SessionSnapshotResponse struct {
	SessionID       string           `json:"sessionId"`
	SnapshotOrdinal string           `json:"snapshotOrdinal,omitempty"`
	Status          string           `json:"status,omitempty"`
	Entities        []SnapshotEntity `json:"entities"`
}
