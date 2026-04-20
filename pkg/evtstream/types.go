package evtstream

import "google.golang.org/protobuf/proto"

// SessionId is the universal routing key used by the substrate.
type SessionId string

// ConnectionId identifies a single transport-level connection.
type ConnectionId string

// Command is the typed request shape entering the substrate.
type Command struct {
	Name         string
	Payload      proto.Message
	SessionId    SessionId
	ConnectionId ConnectionId
}

// Event is the canonical backend event carried through the substrate.
// In phase 1, ordinals are assigned by the in-memory hub path. Later phases
// will move assignment to the bus consumer.
type Event struct {
	Name      string
	Payload   proto.Message
	SessionId SessionId
	Ordinal   uint64
}

// Session is the substrate-owned session object. Metadata is populated by
// SessionMetadataFactory on first reference.
type Session struct {
	Id       SessionId
	Metadata any
}
