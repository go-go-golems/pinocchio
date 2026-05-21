package rpc_test

import (
	"testing"

	chatapprpcv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/rpc/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestRpcLineGeneratedProtoJSONRoundTrip(t *testing.T) {
	line := &chatapprpcv1.RpcLine{
		Version:   1,
		SessionId: "session-1",
		Frame: &chatapprpcv1.RpcLine_Hello{
			Hello: &chatapprpcv1.HelloFrame{
				Protocol:     "pinocchio.chatapp.rpc.v1",
				Server:       "pinocchio",
				Capabilities: []string{"snapshots", "ui_events", "done"},
			},
		},
	}

	b, err := protojson.MarshalOptions{EmitUnpopulated: false, UseProtoNames: false}.Marshal(line)
	if err != nil {
		t.Fatalf("marshal RpcLine: %v", err)
	}

	var decoded chatapprpcv1.RpcLine
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal RpcLine: %v", err)
	}
	if decoded.GetVersion() != 1 {
		t.Fatalf("unexpected version: %d", decoded.GetVersion())
	}
	if decoded.GetSessionId() != "session-1" {
		t.Fatalf("unexpected session id: %q", decoded.GetSessionId())
	}
	if decoded.GetHello().GetProtocol() != "pinocchio.chatapp.rpc.v1" {
		t.Fatalf("unexpected protocol: %q", decoded.GetHello().GetProtocol())
	}
}
