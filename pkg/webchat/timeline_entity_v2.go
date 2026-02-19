package webchat

import (
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

func timelineStructFromMap(m map[string]any) *structpb.Struct {
	if len(m) == 0 {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	st, err := structpb.NewStruct(m)
	if err != nil || st == nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return st
}

func timelineStructFromProtoMessage(msg proto.Message) *structpb.Struct {
	if msg == nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	raw, err := protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   false,
	}.Marshal(msg)
	if err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return timelineStructFromMap(m)
}

func timelineEntityV2(id, kind string, props *structpb.Struct) *timelinepb.TimelineEntityV2 {
	if props == nil {
		props = &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return &timelinepb.TimelineEntityV2{
		Id:    strings.TrimSpace(id),
		Kind:  strings.TrimSpace(kind),
		Props: props,
	}
}

func timelineEntityV2FromProtoMessage(id, kind string, msg proto.Message) *timelinepb.TimelineEntityV2 {
	return timelineEntityV2(id, kind, timelineStructFromProtoMessage(msg))
}
