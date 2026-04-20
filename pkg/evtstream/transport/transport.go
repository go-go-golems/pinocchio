package transport

import (
	"context"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
)

// Transport is the public wire seam used by the substrate.
type Transport interface {
	Start(ctx context.Context, in chan<- IncomingCommand, out <-chan OutgoingMessage) error
}

// IncomingCommand is the transport-neutral payload entering dispatch.
type IncomingCommand struct {
	ConnectionId evtstream.ConnectionId
	SessionId    evtstream.SessionId
	Name         string
	PayloadBytes []byte
}

// OutgoingMessage is the transport-neutral payload leaving projection fan-out.
type OutgoingMessage struct {
	SessionId     evtstream.SessionId
	ConnectionIds []evtstream.ConnectionId
	UIEvent       evtstream.UIEvent
}
