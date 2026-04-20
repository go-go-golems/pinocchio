package app

type CreateSessionRequest struct {
	Profile  string `json:"profile,omitempty"`
	Registry string `json:"registry,omitempty"`
}

type CreateSessionResponse struct {
	SessionID string `json:"sessionId"`
	Profile   string `json:"profile,omitempty"`
}

type SubmitMessageRequest struct {
	Prompt         string `json:"prompt"`
	Profile        string `json:"profile,omitempty"`
	Registry       string `json:"registry,omitempty"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

type SubmitMessageResponse struct {
	SessionID string `json:"sessionId"`
	Accepted  bool   `json:"accepted"`
	Status    string `json:"status"`
	Profile   string `json:"profile,omitempty"`
}

type SnapshotEntity struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Tombstone bool   `json:"tombstone"`
	Payload   any    `json:"payload,omitempty"`
}

type SessionSnapshotResponse struct {
	SessionID string           `json:"sessionId"`
	Ordinal   string           `json:"ordinal"`
	Status    string           `json:"status,omitempty"`
	Entities  []SnapshotEntity `json:"entities"`
}

type errorResponse struct {
	Error string `json:"error"`
}
