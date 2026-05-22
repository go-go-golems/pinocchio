package jsonl

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	chatapprpcv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/rpc/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// UIFanout adapts projected sessionstream UI events to protobuf-defined JSONL
// RpcLine frames.
type UIFanout struct {
	writer    *Writer
	mu        sync.RWMutex
	requestID string
}

var _ sessionstream.UIFanout = (*UIFanout)(nil)

// NewUIFanout creates a sessionstream UIFanout that writes RpcLine JSONL to w.
func NewUIFanout(w io.Writer) (*UIFanout, error) {
	writer, err := NewWriter(w)
	if err != nil {
		return nil, err
	}
	return NewUIFanoutWithWriter(writer)
}

// NewUIFanoutWithWriter creates a UIFanout using an existing RpcLine writer.
func NewUIFanoutWithWriter(writer *Writer) (*UIFanout, error) {
	if writer == nil {
		return nil, fmt.Errorf("jsonl ui fanout writer is nil")
	}
	return &UIFanout{writer: writer}, nil
}

// SetRequestID sets the request id stamped on subsequently written frames.
func (f *UIFanout) SetRequestID(requestID string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requestID = strings.TrimSpace(requestID)
}

func (f *UIFanout) currentRequestID() string {
	if f == nil {
		return ""
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.requestID
}

// PublishUI writes one ui_event RpcLine for every projected sessionstream UI event.
func (f *UIFanout) PublishUI(_ context.Context, sid sessionstream.SessionId, ord uint64, events []sessionstream.UIEvent) error {
	if f == nil || f.writer == nil {
		return fmt.Errorf("jsonl ui fanout is not initialized")
	}
	for i, ev := range events {
		payload, err := packPayload(ev.Payload)
		if err != nil {
			return fmt.Errorf("ui event %d %q: %w", i, ev.Name, err)
		}
		line := &chatapprpcv1.RpcLine{
			Version:   1,
			SessionId: string(sid),
			RequestId: f.currentRequestID(),
			Frame: &chatapprpcv1.RpcLine_UiEvent{
				UiEvent: &chatapprpcv1.UiEventFrame{
					Ordinal: ord,
					Name:    strings.TrimSpace(ev.Name),
					Payload: payload,
				},
			},
		}
		if err := f.writer.WriteLine(line); err != nil {
			return err
		}
	}
	return nil
}

// WriteHello writes the standard hello frame for a session.
func (f *UIFanout) WriteHello(sid sessionstream.SessionId, capabilities []string) error {
	return f.WriteHelloForRequest(sid, f.currentRequestID(), capabilities)
}

// WriteHelloForRequest writes a hello frame with an explicit request id. Most
// hello frames are connection-level and should pass an empty request id.
func (f *UIFanout) WriteHelloForRequest(sid sessionstream.SessionId, requestID string, capabilities []string) error {
	if f == nil || f.writer == nil {
		return fmt.Errorf("jsonl ui fanout is not initialized")
	}
	return f.writer.WriteLine(WithRequestID(NewHelloLine(string(sid), capabilities), requestID))
}

// WriteError writes a structured error frame for a session.
func (f *UIFanout) WriteError(sid sessionstream.SessionId, code string, err error, terminal bool) error {
	return f.WriteErrorForRequest(sid, f.currentRequestID(), code, err, terminal)
}

// WriteErrorForRequest writes a structured error frame with an explicit request
// id. Stdin RPC control requests use this to avoid mutating the active submit's
// request id while cancellation or validation errors are being reported.
func (f *UIFanout) WriteErrorForRequest(sid sessionstream.SessionId, requestID string, code string, err error, terminal bool) error {
	if f == nil || f.writer == nil {
		return fmt.Errorf("jsonl ui fanout is not initialized")
	}
	return f.writer.WriteLine(WithRequestID(NewErrorLine(string(sid), code, err, terminal), requestID))
}

// WriteDone writes the adapter-level done frame for a session.
func (f *UIFanout) WriteDone(sid sessionstream.SessionId, status string) error {
	return f.WriteDoneForRequest(sid, f.currentRequestID(), status)
}

// WriteDoneForRequest writes an adapter-level done frame with an explicit
// request id.
func (f *UIFanout) WriteDoneForRequest(sid sessionstream.SessionId, requestID string, status string) error {
	if f == nil || f.writer == nil {
		return fmt.Errorf("jsonl ui fanout is not initialized")
	}
	return f.writer.WriteLine(WithRequestID(NewDoneLine(string(sid), status), requestID))
}

// WriteSnapshot writes one snapshot frame containing the current sessionstream
// hydration entities.
func (f *UIFanout) WriteSnapshot(snap sessionstream.Snapshot) error {
	return f.WriteSnapshotForRequest(f.currentRequestID(), snap)
}

// WriteSnapshotForRequest writes one snapshot frame with an explicit request id.
func (f *UIFanout) WriteSnapshotForRequest(requestID string, snap sessionstream.Snapshot) error {
	if f == nil || f.writer == nil {
		return fmt.Errorf("jsonl ui fanout is not initialized")
	}
	entities := make([]*chatapprpcv1.SnapshotEntity, 0, len(snap.Entities))
	for i, entity := range snap.Entities {
		payload, err := packPayload(entity.Payload)
		if err != nil {
			return fmt.Errorf("snapshot entity %d %s/%s: %w", i, entity.Kind, entity.Id, err)
		}
		entities = append(entities, &chatapprpcv1.SnapshotEntity{
			Kind:             strings.TrimSpace(entity.Kind),
			Id:               strings.TrimSpace(entity.Id),
			CreatedOrdinal:   entity.CreatedOrdinal,
			LastEventOrdinal: entity.LastEventOrdinal,
			Tombstone:        entity.Tombstone,
			Payload:          payload,
		})
	}
	return f.writer.WriteLine(&chatapprpcv1.RpcLine{
		Version:   1,
		SessionId: string(snap.SessionId),
		RequestId: strings.TrimSpace(requestID),
		Frame: &chatapprpcv1.RpcLine_Snapshot{
			Snapshot: &chatapprpcv1.SnapshotFrame{
				SnapshotOrdinal: snap.SnapshotOrdinal,
				Entities:        entities,
			},
		},
	})
}

// WriteBackendEvent writes an optional canonical backend event frame for debug or
// advanced streams. Default --rpc mode should prefer projected UI events.
func (f *UIFanout) WriteBackendEvent(sid sessionstream.SessionId, ord uint64, name string, payload proto.Message) error {
	if f == nil || f.writer == nil {
		return fmt.Errorf("jsonl ui fanout is not initialized")
	}
	packed, err := packPayload(payload)
	if err != nil {
		return fmt.Errorf("backend event %q: %w", name, err)
	}
	return f.writer.WriteLine(&chatapprpcv1.RpcLine{
		Version:   1,
		SessionId: string(sid),
		RequestId: f.currentRequestID(),
		Frame: &chatapprpcv1.RpcLine_BackendEvent{
			BackendEvent: &chatapprpcv1.BackendEventFrame{
				Ordinal: ord,
				Name:    strings.TrimSpace(name),
				Payload: packed,
			},
		},
	})
}

func packPayload(payload proto.Message) (*anypb.Any, error) {
	if payload == nil {
		return nil, fmt.Errorf("payload is nil")
	}
	packed, err := anypb.New(payload)
	if err != nil {
		return nil, err
	}
	return packed, nil
}
