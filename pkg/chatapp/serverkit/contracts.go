package serverkit

// CreateSessionRequest is the common JSON body for creating a chat session.
// Applications may ignore Profile/Registry when they do not support runtime
// selection.
type CreateSessionRequest struct {
	ApplicationProfile string `json:"application_profile,omitempty"`
	Profile            string `json:"profile,omitempty"`
	Registry           string `json:"registry,omitempty"`
}

type CreateSessionResponse struct {
	SessionID          string `json:"sessionId"`
	ApplicationProfile string `json:"application_profile,omitempty"`
	Profile            string `json:"profile,omitempty"`
	Registry           string `json:"registry,omitempty"`
}

// SubmitMessageRequest is the common JSON body for adding a user prompt to an
// existing chat session.
// AttachmentRef references an attachment previously uploaded through an
// application-owned endpoint. The application resolves ids to chatapp.Attachment
// values (URL, media type, dimensions) before submitting the prompt.
type AttachmentRef struct {
	AttachmentID string `json:"attachment_id"`
}

type SubmitMessageRequest struct {
	Prompt             string          `json:"prompt"`
	Attachments        []AttachmentRef `json:"attachments,omitempty"`
	ApplicationProfile string          `json:"application_profile,omitempty"`
	Profile            string          `json:"profile,omitempty"`
	Registry           string          `json:"registry,omitempty"`
	IdempotencyKey     string          `json:"idempotencyKey,omitempty"`
}

type SubmitMessageResponse struct {
	SessionID string `json:"sessionId"`
	Accepted  bool   `json:"accepted"`
	Status    string `json:"status"`
	Profile   string `json:"profile,omitempty"`
}

type StopSessionResponse struct {
	SessionID string `json:"sessionId"`
	Accepted  bool   `json:"accepted"`
	Status    string `json:"status"`
}

type SnapshotEntity struct {
	Kind             string `json:"kind"`
	ID               string `json:"id"`
	CreatedOrdinal   string `json:"createdOrdinal,omitempty"`
	LastEventOrdinal string `json:"lastEventOrdinal,omitempty"`
	Tombstone        bool   `json:"tombstone"`
	Payload          any    `json:"payload,omitempty"`
	CreatedAt        int64  `json:"createdAt,omitempty"`
}

type SessionSnapshotResponse struct {
	SessionID       string           `json:"sessionId"`
	SnapshotOrdinal string           `json:"snapshotOrdinal,omitempty"`
	Status          string           `json:"status,omitempty"`
	Entities        []SnapshotEntity `json:"entities"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
