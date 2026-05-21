package jsonl

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	chatapprpcv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/rpc/v1"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestUIFanoutPublishUIWritesTypedAnyPayloads(t *testing.T) {
	var buf bytes.Buffer
	fanout, err := NewUIFanout(&buf)
	if err != nil {
		t.Fatalf("NewUIFanout: %v", err)
	}

	events := []sessionstream.UIEvent{
		{Name: "ChatTextPatch", Payload: &chatappv1.ChatTextPatch{MessageId: "msg-1:text:1", StreamId: "msg-1:text:1", Sequence: 7, Text: "hel", Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND, Status: "streaming"}},
		{Name: "ChatTextSegmentFinished", Payload: &chatappv1.ChatTextSegmentFinished{MessageId: "msg-1:text:1", Content: "hello", Status: "finished", Final: true}},
		{Name: "ChatRunFinished", Payload: &chatappv1.ChatRunFinished{MessageId: "msg-1", Status: "finished"}},
	}
	if err := fanout.PublishUI(context.Background(), "session-1", 42, events); err != nil {
		t.Fatalf("PublishUI: %v", err)
	}

	lines := rpcLinesFromBuffer(t, &buf)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	assertAnyPayload(t, lines[0].GetUiEvent().GetPayload(), &chatappv1.ChatTextPatch{}, func(msg proto.Message) {
		patch := msg.(*chatappv1.ChatTextPatch)
		if patch.GetText() != "hel" || patch.GetSequence() != 7 {
			t.Fatalf("unexpected text patch: %+v", patch)
		}
	})
	assertAnyPayload(t, lines[1].GetUiEvent().GetPayload(), &chatappv1.ChatTextSegmentFinished{}, func(msg proto.Message) {
		finished := msg.(*chatappv1.ChatTextSegmentFinished)
		if finished.GetContent() != "hello" || !finished.GetFinal() {
			t.Fatalf("unexpected finished segment: %+v", finished)
		}
	})
	assertAnyPayload(t, lines[2].GetUiEvent().GetPayload(), &chatappv1.ChatRunFinished{}, func(msg proto.Message) {
		finished := msg.(*chatappv1.ChatRunFinished)
		if finished.GetStatus() != "finished" {
			t.Fatalf("unexpected run finished: %+v", finished)
		}
	})
	for _, line := range lines {
		if line.GetSessionId() != "session-1" {
			t.Fatalf("unexpected session id: %q", line.GetSessionId())
		}
		if line.GetUiEvent().GetOrdinal() != 42 {
			t.Fatalf("unexpected ordinal: %d", line.GetUiEvent().GetOrdinal())
		}
	}
}

func TestUIFanoutWriteSnapshotPacksTimelineEntities(t *testing.T) {
	var buf bytes.Buffer
	fanout, err := NewUIFanout(&buf)
	if err != nil {
		t.Fatalf("NewUIFanout: %v", err)
	}

	snap := sessionstream.Snapshot{
		SessionId:       "session-1",
		SnapshotOrdinal: 99,
		Entities: []sessionstream.TimelineEntity{{
			Kind:             "ChatMessage",
			Id:               "msg-1",
			CreatedOrdinal:   1,
			LastEventOrdinal: 98,
			Payload:          &chatappv1.ChatMessageEntity{MessageId: "msg-1", Role: "assistant", Content: "hello", Status: "finished"},
		}},
	}
	if err := fanout.WriteSnapshot(snap); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}

	lines := rpcLinesFromBuffer(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	frame := lines[0].GetSnapshot()
	if frame.GetSnapshotOrdinal() != 99 {
		t.Fatalf("unexpected snapshot ordinal: %d", frame.GetSnapshotOrdinal())
	}
	if len(frame.GetEntities()) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(frame.GetEntities()))
	}
	entity := frame.GetEntities()[0]
	if entity.GetKind() != "ChatMessage" || entity.GetId() != "msg-1" || entity.GetLastEventOrdinal() != 98 {
		t.Fatalf("unexpected entity: %+v", entity)
	}
	assertAnyPayload(t, entity.GetPayload(), &chatappv1.ChatMessageEntity{}, func(msg proto.Message) {
		message := msg.(*chatappv1.ChatMessageEntity)
		if message.GetContent() != "hello" || message.GetRole() != "assistant" {
			t.Fatalf("unexpected message entity: %+v", message)
		}
	})
}

func TestUIFanoutControlFramesAndBackendEvent(t *testing.T) {
	var buf bytes.Buffer
	fanout, err := NewUIFanout(&buf)
	if err != nil {
		t.Fatalf("NewUIFanout: %v", err)
	}
	if err := fanout.WriteHello("session-1", []string{"snapshots"}); err != nil {
		t.Fatalf("WriteHello: %v", err)
	}
	if err := fanout.WriteBackendEvent("session-1", 5, "ChatRunFinished", &chatappv1.ChatRunFinished{MessageId: "msg-1", Status: "finished"}); err != nil {
		t.Fatalf("WriteBackendEvent: %v", err)
	}
	if err := fanout.WriteError("session-1", "runtime", errors.New("boom"), true); err != nil {
		t.Fatalf("WriteError: %v", err)
	}
	if err := fanout.WriteDone("session-1", "failed"); err != nil {
		t.Fatalf("WriteDone: %v", err)
	}

	lines := rpcLinesFromBuffer(t, &buf)
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if lines[0].GetHello().GetProtocol() != ProtocolName {
		t.Fatalf("unexpected hello: %+v", lines[0].GetHello())
	}
	assertAnyPayload(t, lines[1].GetBackendEvent().GetPayload(), &chatappv1.ChatRunFinished{}, func(msg proto.Message) {
		if msg.(*chatappv1.ChatRunFinished).GetStatus() != "finished" {
			t.Fatalf("unexpected backend payload: %+v", msg)
		}
	})
	if lines[2].GetError().GetMessage() != "boom" || !lines[2].GetError().GetTerminal() {
		t.Fatalf("unexpected error: %+v", lines[2].GetError())
	}
	if lines[3].GetDone().GetStatus() != "failed" {
		t.Fatalf("unexpected done: %+v", lines[3].GetDone())
	}
}

func TestUIFanoutRejectsNilPayloads(t *testing.T) {
	var buf bytes.Buffer
	fanout, err := NewUIFanout(&buf)
	if err != nil {
		t.Fatalf("NewUIFanout: %v", err)
	}
	if err := fanout.PublishUI(context.Background(), "session-1", 1, []sessionstream.UIEvent{{Name: "bad"}}); err == nil {
		t.Fatal("expected nil UI payload error")
	}
	if err := fanout.WriteSnapshot(sessionstream.Snapshot{SessionId: "session-1", Entities: []sessionstream.TimelineEntity{{Kind: "bad", Id: "bad"}}}); err == nil {
		t.Fatal("expected nil snapshot payload error")
	}
}

func rpcLinesFromBuffer(t *testing.T, buf *bytes.Buffer) []*chatapprpcv1.RpcLine {
	t.Helper()
	trimmed := strings.TrimSuffix(buf.String(), "\n")
	if strings.TrimSpace(trimmed) == "" {
		return nil
	}
	rawLines := strings.Split(trimmed, "\n")
	out := make([]*chatapprpcv1.RpcLine, 0, len(rawLines))
	for i, raw := range rawLines {
		var line chatapprpcv1.RpcLine
		if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("line %d is not RpcLine JSON: %v\n%s", i, err, raw)
		}
		out = append(out, &line)
	}
	return out
}

func assertAnyPayload(t *testing.T, payload *anypb.Any, target proto.Message, check func(proto.Message)) {
	t.Helper()
	if payload == nil {
		t.Fatal("payload is nil")
	}
	if err := payload.UnmarshalTo(target); err != nil {
		t.Fatalf("unpack payload into %T: %v", target, err)
	}
	check(target)
}
