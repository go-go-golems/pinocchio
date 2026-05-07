package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	geppettoobs "github.com/go-go-golems/geppetto/pkg/observability"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	wstransport "github.com/go-go-golems/sessionstream/pkg/sessionstream/transport/ws"
	"google.golang.org/protobuf/proto"
)

const defaultDebugRecorderMaxRecords = 10000

type DebugRecordKind string

const (
	DebugRecordKindPipeline  DebugRecordKind = "pipeline"
	DebugRecordKindTransport DebugRecordKind = "transport"
	DebugRecordKindGeppetto  DebugRecordKind = "geppetto"
)

type DebugRecord struct {
	Kind      DebugRecordKind `json:"kind"`
	Timestamp time.Time       `json:"timestamp"`

	SessionID    string `json:"sessionId,omitempty"`
	ConnectionID string `json:"connectionId,omitempty"`
	Ordinal      string `json:"ordinal,omitempty"`

	Pipeline  *PipelineDebugRecord  `json:"pipeline,omitempty"`
	Transport *TransportDebugRecord `json:"transport,omitempty"`
	Geppetto  *GeppettoDebugRecord  `json:"geppetto,omitempty"`
}

type DebugReconcileResponse struct {
	SessionID               string   `json:"sessionId"`
	PipelineFanoutOrdinals  []string `json:"pipelineFanoutOrdinals"`
	TransportFanoutOrdinals []string `json:"transportFanoutOrdinals"`
	MissingTransportFanout  []string `json:"missingTransportFanout"`
	ExtraTransportFanout    []string `json:"extraTransportFanout"`
	PipelineRecordCount     int      `json:"pipelineRecordCount"`
	TransportRecordCount    int      `json:"transportRecordCount"`
}

type StreamDebugRecorder struct {
	mu         sync.RWMutex
	maxRecords int
	records    []DebugRecord
}

func NewStreamDebugRecorder(maxRecords int) *StreamDebugRecorder {
	if maxRecords <= 0 {
		maxRecords = defaultDebugRecorderMaxRecords
	}
	return &StreamDebugRecorder{maxRecords: maxRecords}
}

func (r *StreamDebugRecorder) OnPipeline(ctx context.Context, rec sessionstream.PipelineRecord) {
	r.RecordPipeline(ctx, rec)
}

func (r *StreamDebugRecorder) OnTransport(ctx context.Context, rec wstransport.TransportRecord) {
	r.RecordTransport(ctx, rec)
}

func (r *StreamDebugRecorder) OnGeppettoRecord(ctx context.Context, rec geppettoobs.Record) {
	r.RecordGeppetto(ctx, rec)
}

func (r *StreamDebugRecorder) RecordPipeline(_ context.Context, rec sessionstream.PipelineRecord) {
	if r == nil {
		return
	}
	out := DebugRecord{
		Kind:      DebugRecordKindPipeline,
		Timestamp: time.Now().UTC(),
		SessionID: string(rec.SessionId),
		Ordinal:   formatUint(rec.Ordinal),
		Pipeline:  encodePipelineRecord(rec),
	}
	r.append(out)
}

func (r *StreamDebugRecorder) RecordTransport(_ context.Context, rec wstransport.TransportRecord) {
	if r == nil {
		return
	}
	out := DebugRecord{
		Kind:         DebugRecordKindTransport,
		Timestamp:    time.Now().UTC(),
		SessionID:    string(rec.SessionId),
		ConnectionID: string(rec.ConnectionId),
		Ordinal:      formatUint(rec.Ordinal),
		Transport:    encodeTransportRecord(rec),
	}
	r.append(out)
}

func (r *StreamDebugRecorder) RecordGeppetto(_ context.Context, rec geppettoobs.Record) {
	if r == nil {
		return
	}
	timestamp := rec.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	out := DebugRecord{
		Kind:      DebugRecordKindGeppetto,
		Timestamp: timestamp,
		SessionID: rec.SessionID,
		Geppetto:  encodeGeppettoRecord(rec),
	}
	r.append(out)
}

func (r *StreamDebugRecorder) Records(sessionID string, kind DebugRecordKind) []DebugRecord {
	if r == nil {
		return nil
	}
	sessionID = strings.TrimSpace(sessionID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]DebugRecord, 0, len(r.records))
	for _, rec := range r.records {
		if sessionID != "" && rec.SessionID != sessionID {
			continue
		}
		if kind != "" && rec.Kind != kind {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *StreamDebugRecorder) append(rec DebugRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
	if len(r.records) > r.maxRecords {
		copy(r.records, r.records[len(r.records)-r.maxRecords:])
		r.records = r.records[:r.maxRecords]
	}
}

func (r *StreamDebugRecorder) Reconcile(sessionID string) DebugReconcileResponse {
	records := r.Records(sessionID, "")
	pipeline := map[string]bool{}
	transport := map[string]bool{}
	pipelineCount := 0
	transportCount := 0
	for _, rec := range records {
		switch rec.Kind {
		case DebugRecordKindPipeline:
			pipelineCount++
			if rec.Pipeline != nil && len(rec.Pipeline.FanoutEvents) > 0 && rec.Ordinal != "" {
				pipeline[rec.Ordinal] = true
			}
		case DebugRecordKindTransport:
			transportCount++
			if rec.Transport != nil && rec.Transport.Stage == "fanout_started" && rec.Ordinal != "" {
				transport[rec.Ordinal] = true
			}
		}
	}
	return DebugReconcileResponse{
		SessionID:               sessionID,
		PipelineFanoutOrdinals:  sortedKeys(pipeline),
		TransportFanoutOrdinals: sortedKeys(transport),
		MissingTransportFanout:  missingKeys(pipeline, transport),
		ExtraTransportFanout:    missingKeys(transport, pipeline),
		PipelineRecordCount:     pipelineCount,
		TransportRecordCount:    transportCount,
	}
}

func protoType(msg proto.Message) string {
	if msg == nil {
		return ""
	}
	return string(msg.ProtoReflect().Descriptor().FullName())
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func formatUint(v uint64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%d", v)
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return numericStringLess(out[i], out[j]) })
	return out
}

func missingKeys(left, right map[string]bool) []string {
	if len(left) == 0 {
		return nil
	}
	out := make([]string, 0)
	for k := range left {
		if !right[k] {
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool { return numericStringLess(out[i], out[j]) })
	return out
}

func numericStringLess(a, b string) bool {
	ai, aerr := strconv.ParseUint(a, 10, 64)
	bi, berr := strconv.ParseUint(b, 10, 64)
	if aerr == nil && berr == nil {
		return ai < bi
	}
	return a < b
}
