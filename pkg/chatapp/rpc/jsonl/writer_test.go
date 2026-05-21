package jsonl

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"

	chatapprpcv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/rpc/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestWriterWriteLineWritesOneProtoJSONLine(t *testing.T) {
	var buf bytes.Buffer
	w := MustNewWriter(&buf)

	if err := w.WriteLine(NewHelloLine("session-1", []string{"snapshots", "ui_events"})); err != nil {
		t.Fatalf("WriteLine: %v", err)
	}

	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected trailing newline, got %q", out)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one line, got %d: %q", len(lines), out)
	}

	var decoded chatapprpcv1.RpcLine
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("unmarshal line: %v", err)
	}
	if decoded.GetVersion() != 1 {
		t.Fatalf("unexpected version: %d", decoded.GetVersion())
	}
	if decoded.GetSessionId() != "session-1" {
		t.Fatalf("unexpected session id: %q", decoded.GetSessionId())
	}
	if decoded.GetHello().GetProtocol() != ProtocolName {
		t.Fatalf("unexpected protocol: %q", decoded.GetHello().GetProtocol())
	}
}

func TestWriterRejectsInvalidInput(t *testing.T) {
	if _, err := NewWriter(nil); err == nil {
		t.Fatal("expected NewWriter(nil) to fail")
	}

	var buf bytes.Buffer
	w := MustNewWriter(&buf)
	if err := w.WriteLine(nil); err == nil {
		t.Fatal("expected WriteLine(nil) to fail")
	}
	if got := buf.String(); got != "" {
		t.Fatalf("expected no output for nil line, got %q", got)
	}
}

func TestWriterConcurrentWritesProduceCompleteLines(t *testing.T) {
	var buf bytes.Buffer
	w := MustNewWriter(&buf)

	const count = 25
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			if err := w.WriteLine(NewDoneLine("session-1", "finished")); err != nil {
				t.Errorf("WriteLine: %v", err)
			}
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != count {
		t.Fatalf("expected %d lines, got %d", count, len(lines))
	}
	for i, line := range lines {
		var decoded chatapprpcv1.RpcLine
		if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("line %d is not valid RpcLine JSON: %v\n%s", i, err, line)
		}
		if decoded.GetDone().GetStatus() != "finished" {
			t.Fatalf("line %d unexpected status: %q", i, decoded.GetDone().GetStatus())
		}
	}
}

func TestFrameConstructors(t *testing.T) {
	errLine := NewErrorLine("session-1", "boom", errors.New("failed"), true)
	if errLine.GetError().GetCode() != "boom" {
		t.Fatalf("unexpected error code: %q", errLine.GetError().GetCode())
	}
	if errLine.GetError().GetMessage() != "failed" {
		t.Fatalf("unexpected error message: %q", errLine.GetError().GetMessage())
	}
	if !errLine.GetError().GetTerminal() {
		t.Fatal("expected terminal error")
	}

	doneLine := NewDoneLine("session-1", "finished")
	if doneLine.GetDone().GetStatus() != "finished" {
		t.Fatalf("unexpected done status: %q", doneLine.GetDone().GetStatus())
	}
}
