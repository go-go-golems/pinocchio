package jsonl

import (
	"fmt"
	"io"
	"strings"
	"sync"

	chatapprpcv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/rpc/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	ProtocolName = "pinocchio.chatapp.rpc.v1"
	ServerName   = "pinocchio"
)

var defaultMarshalOptions = protojson.MarshalOptions{
	EmitUnpopulated: false,
	UseProtoNames:   false,
}

// Writer writes one protobuf JSON RpcLine per line.
//
// Writer is safe for concurrent use. It serializes each protobuf message before
// acquiring the write lock, then writes the complete JSON document plus trailing
// newline while holding the lock so concurrent callers cannot interleave lines.
type Writer struct {
	w       io.Writer
	mu      sync.Mutex
	marshal protojson.MarshalOptions
}

// NewWriter creates a Writer that emits protojson-encoded RpcLine messages to w.
func NewWriter(w io.Writer) (*Writer, error) {
	if w == nil {
		return nil, fmt.Errorf("jsonl writer output is nil")
	}
	return &Writer{w: w, marshal: defaultMarshalOptions}, nil
}

// MustNewWriter creates a Writer or panics. It is intended for tests and package
// initialization paths where a nil writer is a programmer error.
func MustNewWriter(w io.Writer) *Writer {
	writer, err := NewWriter(w)
	if err != nil {
		panic(err)
	}
	return writer
}

// WriteLine writes line as one complete protobuf JSON object followed by a
// newline. Nil lines are rejected so callers do not accidentally emit "{}".
func (w *Writer) WriteLine(line *chatapprpcv1.RpcLine) error {
	if w == nil || w.w == nil {
		return fmt.Errorf("jsonl writer is not initialized")
	}
	if line == nil {
		return fmt.Errorf("rpc line is nil")
	}
	b, err := w.marshal.Marshal(line)
	if err != nil {
		return err
	}
	b = append(b, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = w.w.Write(b)
	return err
}

// NewHelloLine returns the standard hello frame for a Pinocchio chatapp RPC
// stream.
func NewHelloLine(sessionID string, capabilities []string) *chatapprpcv1.RpcLine {
	return &chatapprpcv1.RpcLine{
		Version:   1,
		SessionId: strings.TrimSpace(sessionID),
		Frame: &chatapprpcv1.RpcLine_Hello{
			Hello: &chatapprpcv1.HelloFrame{
				Protocol:     ProtocolName,
				Server:       ServerName,
				Capabilities: append([]string(nil), capabilities...),
			},
		},
	}
}

// NewErrorLine returns a structured error frame. If err is nil, message is left
// empty and code/detail still identify the error class.
func NewErrorLine(sessionID string, code string, err error, terminal bool) *chatapprpcv1.RpcLine {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return &chatapprpcv1.RpcLine{
		Version:   1,
		SessionId: strings.TrimSpace(sessionID),
		Frame: &chatapprpcv1.RpcLine_Error{
			Error: &chatapprpcv1.ErrorFrame{
				Code:     strings.TrimSpace(code),
				Message:  message,
				Detail:   message,
				Terminal: terminal,
			},
		},
	}
}

// NewDoneLine returns a terminal adapter-level done frame.
func NewDoneLine(sessionID string, status string) *chatapprpcv1.RpcLine {
	return &chatapprpcv1.RpcLine{
		Version:   1,
		SessionId: strings.TrimSpace(sessionID),
		Frame: &chatapprpcv1.RpcLine_Done{
			Done: &chatapprpcv1.DoneFrame{Status: strings.TrimSpace(status)},
		},
	}
}
