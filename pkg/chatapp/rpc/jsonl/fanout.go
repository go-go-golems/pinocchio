package jsonl

import (
	"context"
	"fmt"
	"io"
	"strings"

	chatapprpcv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/rpc/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// UIFanout adapts projected sessionstream UI events to protobuf-defined JSONL
// RpcLine frames.
type UIFanout struct {
	writer *Writer
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
	if f == nil || f.writer == nil {
		return fmt.Errorf("jsonl ui fanout is not initialized")
	}
	return f.writer.WriteLine(NewHelloLine(string(sid), capabilities))
}

// WriteError writes a structured error frame for a session.
func (f *UIFanout) WriteError(sid sessionstream.SessionId, code string, err error, terminal bool) error {
	if f == nil || f.writer == nil {
		return fmt.Errorf("jsonl ui fanout is not initialized")
	}
	return f.writer.WriteLine(NewErrorLine(string(sid), code, err, terminal))
}

// WriteDone writes the adapter-level done frame for a session.
func (f *UIFanout) WriteDone(sid sessionstream.SessionId, status string) error {
	if f == nil || f.writer == nil {
		return fmt.Errorf("jsonl ui fanout is not initialized")
	}
	return f.writer.WriteLine(NewDoneLine(string(sid), status))
}

// WriteSnapshot writes one snapshot frame containing the current sessionstream
// hydration entities.
func (f *UIFanout) WriteSnapshot(snap sessionstream.Snapshot) error {
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
