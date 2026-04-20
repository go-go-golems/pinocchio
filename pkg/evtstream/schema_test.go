package evtstream

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestSchemaRegistryRejectsDuplicateRegistration(t *testing.T) {
	r := NewSchemaRegistry()
	require.NoError(t, r.RegisterCommand("LabStart", &structpb.Struct{}))
	require.Error(t, r.RegisterCommand("LabStart", &structpb.Struct{}))
}

func TestSchemaRegistryDecodeCommandJSON(t *testing.T) {
	r := NewSchemaRegistry()
	require.NoError(t, r.RegisterCommand("LabStart", &structpb.Struct{}))
	msg, err := r.DecodeCommandJSON("LabStart", []byte(`{"prompt":"hello"}`))
	require.NoError(t, err)
	payload := msg.(*structpb.Struct).AsMap()
	require.Equal(t, "hello", payload["prompt"])
}
