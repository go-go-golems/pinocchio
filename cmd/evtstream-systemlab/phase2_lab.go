package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	sessionstream "github.com/go-go-golems/sessionstream"
	storememory "github.com/go-go-golems/sessionstream/hydration/memory"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	phase2CommandName    = "PublishOrderedEvent"
	phase2EventName      = "OrderedEvent"
	phase2UIEventName    = "OrderedEventObserved"
	phase2TimelineEntity = "OrderedEventRecord"
)

type phase2RunRequest struct {
	Action     string `json:"action"`
	SessionA   string `json:"sessionA"`
	SessionB   string `json:"sessionB"`
	BurstCount int    `json:"burstCount"`
	StreamMode string `json:"streamMode"`
}

type phase2RunResponse struct {
	Action             string                    `json:"action"`
	SessionA           string                    `json:"sessionA"`
	SessionB           string                    `json:"sessionB"`
	BurstCount         int                       `json:"burstCount"`
	StreamMode         string                    `json:"streamMode"`
	Trace              []traceEntry              `json:"trace"`
	MessageHistory     []map[string]any          `json:"messageHistory"`
	PerSessionOrdinals map[string][]string       `json:"perSessionOrdinals"`
	Fanout             map[string][]namedPayload `json:"fanout"`
	Snapshots          map[string]map[string]any `json:"snapshots"`
	Checks             map[string]bool           `json:"checks"`
	Error              string                    `json:"error,omitempty"`
}

type phase2MessageRecord struct {
	MessageID        string            `json:"messageId"`
	SessionID        string            `json:"sessionId"`
	EventName        string            `json:"eventName"`
	Label            string            `json:"label,omitempty"`
	Topic            string            `json:"topic"`
	PublishedOrdinal uint64            `json:"publishedOrdinal"`
	AssignedOrdinal  uint64            `json:"assignedOrdinal"`
	PublishMetadata  map[string]string `json:"publishMetadata,omitempty"`
	ConsumeMetadata  map[string]string `json:"consumeMetadata,omitempty"`
}

type phase2State struct {
	hub               *sessionstream.Hub
	store             *storememory.Store
	cancel            context.CancelFunc
	streamMode        string
	syntheticSequence uint64
	publishCounters   map[string]int
	sessionMeta       map[string]map[string]any
	trace             []traceEntry
	messages          map[string]*phase2MessageRecord
	messageOrder      []string
	ordinals          map[string][]uint64
	fanout            map[string][]namedPayload
	lastRun           phase2RunResponse
}

func (e *labEnvironment) newPhase2State() (*phase2State, error) {
	state := &phase2State{
		streamMode:      "derived",
		publishCounters: map[string]int{},
		sessionMeta:     map[string]map[string]any{},
		messages:        map[string]*phase2MessageRecord{},
		messageOrder:    []string{},
		ordinals:        map[string][]uint64{},
		fanout:          map[string][]namedPayload{},
	}
	store := storememory.New()
	reg := sessionstream.NewSchemaRegistry()
	for _, err := range []error{
		reg.RegisterCommand(phase2CommandName, &structpb.Struct{}),
		reg.RegisterEvent(phase2EventName, &structpb.Struct{}),
		reg.RegisterUIEvent(phase2UIEventName, &structpb.Struct{}),
		reg.RegisterTimelineEntity(phase2TimelineEntity, &structpb.Struct{}),
	} {
		if err != nil {
			return nil, err
		}
	}

	pubsub := gochannel.NewGoChannel(gochannel.Config{OutputChannelBuffer: 256}, watermill.NopLogger{})
	hub, err := sessionstream.NewHub(
		sessionstream.WithSchemaRegistry(reg),
		sessionstream.WithHydrationStore(store),
		sessionstream.WithSessionMetadataFactory(func(_ context.Context, sid sessionstream.SessionId) (any, error) {
			meta := map[string]any{
				"sessionId": string(sid),
				"createdBy": "evtstream-systemlab",
				"lab":       "phase2",
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.phase2 != nil {
				e.phase2.sessionMeta[string(sid)] = cloneMap(meta)
				e.phase2AppendTraceLocked("session", "phase 2 session created", meta)
			}
			return cloneMap(meta), nil
		}),
		sessionstream.WithUIFanout(sessionstream.UIFanoutFunc(func(_ context.Context, sid sessionstream.SessionId, ord uint64, events []sessionstream.UIEvent) error {
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.phase2 == nil {
				return nil
			}
			for _, uiEvent := range events {
				payload := protoStructMap(uiEvent.Payload)
				payload["ordinal"] = fmt.Sprintf("%d", ord)
				e.phase2.fanout[string(sid)] = append(e.phase2.fanout[string(sid)], namedPayload{Name: uiEvent.Name, Payload: payload})
			}
			return nil
		})),
		sessionstream.WithEventBus(pubsub, pubsub,
			sessionstream.WithBusTopic("sessionstream.phase2"),
			sessionstream.WithBusMessageMutator(e.phase2MessageMutator),
			sessionstream.WithBusObserver(sessionstream.BusObserverHooks{
				OnPublished: e.phase2Published,
				OnConsumed:  e.phase2Consumed,
			}),
		),
	)
	if err != nil {
		return nil, err
	}
	if err := hub.RegisterCommand(phase2CommandName, e.handlePhase2Command); err != nil {
		return nil, err
	}
	if err := hub.RegisterUIProjection(sessionstream.UIProjectionFunc(e.phase2UIProjection)); err != nil {
		return nil, err
	}
	if err := hub.RegisterTimelineProjection(sessionstream.TimelineProjectionFunc(e.phase2TimelineProjection)); err != nil {
		return nil, err
	}

	state.hub = hub
	state.store = store
	return state, nil
}

func (e *labEnvironment) startPhase2() error {
	e.mu.Lock()
	state := e.phase2
	e.mu.Unlock()
	if state == nil || state.hub == nil {
		return fmt.Errorf("phase 2 state is not initialized")
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := state.hub.Run(ctx); err != nil {
		cancel()
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.phase2 != state {
		cancel()
		return nil
	}
	state.cancel = cancel
	state.trace = nil
	state.messages = map[string]*phase2MessageRecord{}
	state.messageOrder = nil
	state.ordinals = map[string][]uint64{}
	state.fanout = map[string][]namedPayload{}
	state.publishCounters = map[string]int{}
	state.syntheticSequence = 0
	state.sessionMeta = map[string]map[string]any{}
	e.phase2AppendTraceLocked("consumer", "phase 2 consumer started", map[string]any{"topic": "sessionstream.phase2"})
	return nil
}

func (e *labEnvironment) shutdownPhase2() error {
	e.mu.Lock()
	state := e.phase2
	e.phase2 = nil
	e.mu.Unlock()
	if state == nil || state.hub == nil {
		return nil
	}
	if state.cancel != nil {
		state.cancel()
	}
	return state.hub.Shutdown(context.Background())
}

func (e *labEnvironment) restartPhase2Consumer() error {
	e.mu.Lock()
	state := e.phase2
	e.mu.Unlock()
	if state == nil || state.hub == nil {
		return fmt.Errorf("phase 2 state is not initialized")
	}
	if state.cancel != nil {
		state.cancel()
	}
	if err := state.hub.Shutdown(context.Background()); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := state.hub.Run(ctx); err != nil {
		cancel()
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.phase2 != state {
		cancel()
		return nil
	}
	state.cancel = cancel
	e.phase2AppendTraceLocked("consumer", "phase 2 consumer restarted", nil)
	return nil
}

func (e *labEnvironment) RunPhase2(ctx context.Context, in phase2RunRequest) (phase2RunResponse, error) {
	action := strings.TrimSpace(in.Action)
	if action == "" {
		action = "publish-a"
	}
	sessionA := strings.TrimSpace(in.SessionA)
	if sessionA == "" {
		sessionA = "s-a"
	}
	sessionB := strings.TrimSpace(in.SessionB)
	if sessionB == "" {
		sessionB = "s-b"
	}
	burstCount := in.BurstCount
	if burstCount <= 0 {
		burstCount = 4
	}
	streamMode := normalizeStreamMode(in.StreamMode)

	e.mu.Lock()
	if e.phase2 == nil {
		e.mu.Unlock()
		return phase2RunResponse{}, fmt.Errorf("phase 2 state is not initialized")
	}
	e.phase2.streamMode = streamMode
	beforeA := len(e.phase2.ordinals[sessionA])
	beforeB := len(e.phase2.ordinals[sessionB])
	e.phase2AppendTraceLocked("control", "phase 2 action requested", map[string]any{
		"action":     action,
		"sessionA":   sessionA,
		"sessionB":   sessionB,
		"burstCount": burstCount,
		"streamMode": streamMode,
	})
	hub := e.phase2.hub
	e.mu.Unlock()

	var err error
	switch action {
	case "publish-a":
		err = e.submitPhase2Command(ctx, hub, sessionstream.SessionId(sessionA), e.nextPhase2Label(sessionA))
		if err == nil {
			err = e.waitForPhase2Consumed(sessionA, beforeA+1)
		}
	case "publish-b":
		err = e.submitPhase2Command(ctx, hub, sessionstream.SessionId(sessionB), e.nextPhase2Label(sessionB))
		if err == nil {
			err = e.waitForPhase2Consumed(sessionB, beforeB+1)
		}
	case "burst-a":
		for i := 0; i < burstCount; i++ {
			if err = e.submitPhase2Command(ctx, hub, sessionstream.SessionId(sessionA), e.nextPhase2Label(sessionA)); err != nil {
				break
			}
		}
		if err == nil {
			err = e.waitForPhase2Consumed(sessionA, beforeA+burstCount)
		}
	case "restart-consumer":
		err = e.restartPhase2Consumer()
	case "reset-phase2":
		err = e.resetPhase2Only()
	default:
		err = fmt.Errorf("unknown phase 2 action %q", action)
	}

	resp, buildErr := e.buildPhase2Response(action, sessionA, sessionB, burstCount, streamMode)
	if buildErr != nil {
		return phase2RunResponse{}, buildErr
	}
	if err != nil {
		resp.Error = err.Error()
	}

	e.mu.Lock()
	if e.phase2 != nil {
		e.phase2.lastRun = clonePhase2RunResponse(resp)
	}
	e.mu.Unlock()
	return resp, nil
}

func (e *labEnvironment) resetPhase2Only() error {
	e.mu.Lock()
	state := e.phase2
	e.mu.Unlock()
	if state == nil {
		return fmt.Errorf("phase 2 state is not initialized")
	}
	if state.cancel != nil {
		state.cancel()
	}
	if err := state.hub.Shutdown(context.Background()); err != nil {
		return err
	}
	newState, err := e.newPhase2State()
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.phase2 = newState
	e.mu.Unlock()
	return e.startPhase2()
}

func (e *labEnvironment) submitPhase2Command(ctx context.Context, hub *sessionstream.Hub, sid sessionstream.SessionId, label string) error {
	payload, err := structpb.NewStruct(map[string]any{"label": label})
	if err != nil {
		return err
	}
	return hub.Submit(ctx, sid, phase2CommandName, payload)
}

func (e *labEnvironment) nextPhase2Label(sessionID string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.phase2 == nil {
		return sessionID + "-1"
	}
	e.phase2.publishCounters[sessionID]++
	return fmt.Sprintf("%s-%02d", sessionID, e.phase2.publishCounters[sessionID])
}

func (e *labEnvironment) waitForPhase2Consumed(sessionID string, want int) error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		count := 0
		if e.phase2 != nil {
			count = len(e.phase2.ordinals[sessionID])
		}
		e.mu.Unlock()
		if count >= want {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for phase 2 consumed count %d for session %q", want, sessionID)
}

func (e *labEnvironment) buildPhase2Response(action, sessionA, sessionB string, burstCount int, streamMode string) (phase2RunResponse, error) {
	e.mu.Lock()
	state := e.phase2
	if state == nil {
		e.mu.Unlock()
		return phase2RunResponse{}, fmt.Errorf("phase 2 state is not initialized")
	}
	trace := append([]traceEntry(nil), state.trace...)
	messageHistoryRaw := make([]phase2MessageRecord, 0, len(state.messageOrder))
	for _, id := range state.messageOrder {
		if record := state.messages[id]; record != nil {
			messageHistoryRaw = append(messageHistoryRaw, clonePhase2MessageRecord(*record))
		}
	}
	ordinalsRaw := clonePhase2Ordinals(state.ordinals)
	fanout := cloneNamedPayloadMap(state.fanout)
	hub := state.hub
	e.mu.Unlock()

	snapshots := map[string]map[string]any{}
	for _, sid := range uniqueStrings(sessionA, sessionB) {
		snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId(sid))
		if err != nil {
			return phase2RunResponse{}, err
		}
		encoded := encodeSnapshot(snap)
		encoded["ordinal"] = fmt.Sprintf("%d", snap.Ordinal)
		snapshots[sid] = encoded
	}

	resp := phase2RunResponse{
		Action:             action,
		SessionA:           sessionA,
		SessionB:           sessionB,
		BurstCount:         burstCount,
		StreamMode:         streamMode,
		Trace:              trace,
		MessageHistory:     phase2MessageHistoryView(messageHistoryRaw),
		PerSessionOrdinals: phase2OrdinalStrings(ordinalsRaw),
		Fanout:             fanout,
		Snapshots:          snapshots,
		Checks: map[string]bool{
			"publishOrdinalZero":  phase2PublishOrdinalsZero(messageHistoryRaw),
			"monotonicPerSession": phase2Monotonic(ordinalsRaw),
			"sessionIsolation":    phase2SessionIsolation(ordinalsRaw),
			"messagesConsumed":    phase2ConsumedCount(messageHistoryRaw) > 0,
		},
	}
	return resp, nil
}

func (e *labEnvironment) ExportPhase2(format string) (string, string, []byte, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "json"
	}
	e.mu.Lock()
	state := e.phase2
	if state == nil {
		e.mu.Unlock()
		return "", "", nil, fmt.Errorf("phase 2 state is not initialized")
	}
	resp := clonePhase2RunResponse(state.lastRun)
	e.mu.Unlock()
	if resp.Action == "" {
		return "", "", nil, fmt.Errorf("no phase 2 transcript available")
	}
	switch format {
	case "json":
		body, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return "", "", nil, err
		}
		return "phase2-transcript.json", "application/json", body, nil
	case "md", "markdown":
		return "phase2-transcript.md", "text/markdown; charset=utf-8", []byte(renderPhase2Markdown(resp)), nil
	default:
		return "", "", nil, fmt.Errorf("unsupported export format %q", format)
	}
}

func (e *labEnvironment) handlePhase2Command(ctx context.Context, cmd sessionstream.Command, sess *sessionstream.Session, pub sessionstream.EventPublisher) error {
	payload := protoStructMap(cmd.Payload)
	label := strings.TrimSpace(toString(payload["label"]))
	if label == "" {
		label = fmt.Sprintf("%s-%d", cmd.SessionId, time.Now().UnixNano())
	}
	e.mu.Lock()
	e.phase2AppendTraceLocked("handler", "phase 2 handler invoked", map[string]any{
		"sessionId":  string(cmd.SessionId),
		"label":      label,
		"hasSession": sess != nil,
	})
	e.mu.Unlock()
	eventPayload, err := structpb.NewStruct(map[string]any{
		"label":     label,
		"sessionId": string(cmd.SessionId),
	})
	if err != nil {
		return err
	}
	return pub.Publish(ctx, sessionstream.Event{Name: phase2EventName, SessionId: cmd.SessionId, Payload: eventPayload})
}

func (e *labEnvironment) phase2UIProjection(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, error) {
	payload := protoStructMap(ev.Payload)
	payload["ordinal"] = fmt.Sprintf("%d", ev.Ordinal)
	pb, err := structpb.NewStruct(payload)
	if err != nil {
		return nil, err
	}
	return []sessionstream.UIEvent{{Name: phase2UIEventName, Payload: pb}}, nil
}

func (e *labEnvironment) phase2TimelineProjection(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.TimelineEntity, error) {
	payload := protoStructMap(ev.Payload)
	label := toString(payload["label"])
	entityPayload, err := structpb.NewStruct(map[string]any{
		"label":     label,
		"sessionId": payload["sessionId"],
		"ordinal":   fmt.Sprintf("%d", ev.Ordinal),
	})
	if err != nil {
		return nil, err
	}
	return []sessionstream.TimelineEntity{{Kind: phase2TimelineEntity, Id: label, Payload: entityPayload}}, nil
}

func (e *labEnvironment) phase2MessageMutator(_ context.Context, _ sessionstream.Event, msg *message.Message) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.phase2 == nil {
		return nil
	}
	e.phase2.syntheticSequence++
	seq := e.phase2.syntheticSequence
	switch e.phase2.streamMode {
	case "missing":
		return nil
	case "invalid":
		msg.Metadata.Set(sessionstream.MetadataKeyStreamID, fmt.Sprintf("invalid-%d", seq))
	default:
		msg.Metadata.Set(sessionstream.MetadataKeyStreamID, fmt.Sprintf("1713560000123-%d", seq))
	}
	return nil
}

func (e *labEnvironment) phase2Published(_ context.Context, ev sessionstream.Event, rec sessionstream.BusRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.phase2 == nil {
		return
	}
	record := e.phase2RecordLocked(rec.MessageID)
	record.MessageID = rec.MessageID
	record.SessionID = string(ev.SessionId)
	record.EventName = ev.Name
	record.Label = toString(protoStructMap(ev.Payload)["label"])
	record.Topic = rec.Topic
	record.PublishedOrdinal = ev.Ordinal
	record.PublishMetadata = cloneStringMap(rec.Metadata)
	e.phase2AppendTraceLocked("publish", "phase 2 event published", map[string]any{
		"messageId": rec.MessageID,
		"sessionId": record.SessionID,
		"streamId":  rec.Metadata[sessionstream.MetadataKeyStreamID],
	})
}

func (e *labEnvironment) phase2Consumed(_ context.Context, ev sessionstream.Event, rec sessionstream.BusRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.phase2 == nil {
		return
	}
	record := e.phase2RecordLocked(rec.MessageID)
	record.MessageID = rec.MessageID
	record.SessionID = string(ev.SessionId)
	record.EventName = ev.Name
	record.Label = toString(protoStructMap(ev.Payload)["label"])
	record.Topic = rec.Topic
	record.AssignedOrdinal = ev.Ordinal
	record.ConsumeMetadata = cloneStringMap(rec.Metadata)
	e.phase2.ordinals[string(ev.SessionId)] = append(e.phase2.ordinals[string(ev.SessionId)], ev.Ordinal)
	e.phase2AppendTraceLocked("consume", "phase 2 event consumed", map[string]any{
		"messageId": rec.MessageID,
		"sessionId": string(ev.SessionId),
		"ordinal":   fmt.Sprintf("%d", ev.Ordinal),
		"streamId":  rec.Metadata[sessionstream.MetadataKeyStreamID],
	})
}

func (e *labEnvironment) phase2RecordLocked(messageID string) *phase2MessageRecord {
	record := e.phase2.messages[messageID]
	if record == nil {
		record = &phase2MessageRecord{}
		e.phase2.messages[messageID] = record
		e.phase2.messageOrder = append(e.phase2.messageOrder, messageID)
	}
	return record
}

func (e *labEnvironment) phase2AppendTraceLocked(kind, message string, details map[string]any) {
	step := len(e.phase2.trace) + 1
	e.phase2.trace = append(e.phase2.trace, traceEntry{Step: step, Kind: kind, Message: message, Details: cloneMap(details)})
}

func normalizeStreamMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "missing", "invalid":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "derived"
	}
}

func phase2PublishOrdinalsZero(history []phase2MessageRecord) bool {
	if len(history) == 0 {
		return false
	}
	for _, record := range history {
		if record.PublishedOrdinal != 0 {
			return false
		}
	}
	return true
}

func phase2Monotonic(ordinals map[string][]uint64) bool {
	if len(ordinals) == 0 {
		return false
	}
	for _, values := range ordinals {
		for i := 1; i < len(values); i++ {
			if values[i] <= values[i-1] {
				return false
			}
		}
	}
	return true
}

func phase2SessionIsolation(ordinals map[string][]uint64) bool {
	if len(ordinals) == 0 {
		return false
	}
	for _, values := range ordinals {
		if len(values) == 0 {
			return false
		}
	}
	return true
}

func phase2ConsumedCount(history []phase2MessageRecord) int {
	count := 0
	for _, record := range history {
		if record.AssignedOrdinal > 0 {
			count++
		}
	}
	return count
}

func clonePhase2MessageRecord(in phase2MessageRecord) phase2MessageRecord {
	out := in
	out.PublishMetadata = cloneStringMap(in.PublishMetadata)
	out.ConsumeMetadata = cloneStringMap(in.ConsumeMetadata)
	return out
}

func clonePhase2Ordinals(in map[string][]uint64) map[string][]uint64 {
	if in == nil {
		return nil
	}
	out := make(map[string][]uint64, len(in))
	for sid, values := range in {
		out[sid] = append([]uint64(nil), values...)
	}
	return out
}

func cloneNamedPayloadMap(in map[string][]namedPayload) map[string][]namedPayload {
	if in == nil {
		return nil
	}
	out := make(map[string][]namedPayload, len(in))
	for sid, values := range in {
		cloned := make([]namedPayload, 0, len(values))
		for _, value := range values {
			cloned = append(cloned, namedPayload{Name: value.Name, Payload: cloneMap(value.Payload)})
		}
		out[sid] = cloned
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func clonePhase2RunResponse(in phase2RunResponse) phase2RunResponse {
	out := in
	out.Trace = append([]traceEntry(nil), in.Trace...)
	out.MessageHistory = make([]map[string]any, 0, len(in.MessageHistory))
	for _, record := range in.MessageHistory {
		out.MessageHistory = append(out.MessageHistory, cloneMap(record))
	}
	out.PerSessionOrdinals = cloneStringSlicesMap(in.PerSessionOrdinals)
	out.Fanout = cloneNamedPayloadMap(in.Fanout)
	out.Snapshots = make(map[string]map[string]any, len(in.Snapshots))
	for sid, snap := range in.Snapshots {
		out.Snapshots[sid] = cloneMap(snap)
	}
	out.Checks = cloneBoolMap(in.Checks)
	return out
}

func phase2MessageHistoryView(history []phase2MessageRecord) []map[string]any {
	out := make([]map[string]any, 0, len(history))
	for _, record := range history {
		out = append(out, map[string]any{
			"messageId":        record.MessageID,
			"sessionId":        record.SessionID,
			"eventName":        record.EventName,
			"label":            record.Label,
			"topic":            record.Topic,
			"publishedOrdinal": fmt.Sprintf("%d", record.PublishedOrdinal),
			"assignedOrdinal":  fmt.Sprintf("%d", record.AssignedOrdinal),
			"publishMetadata":  cloneStringMap(record.PublishMetadata),
			"consumeMetadata":  cloneStringMap(record.ConsumeMetadata),
		})
	}
	return out
}

func phase2OrdinalStrings(in map[string][]uint64) map[string][]string {
	out := make(map[string][]string, len(in))
	for sid, values := range in {
		converted := make([]string, 0, len(values))
		for _, value := range values {
			converted = append(converted, fmt.Sprintf("%d", value))
		}
		out[sid] = converted
	}
	return out
}

func cloneStringSlicesMap(in map[string][]string) map[string][]string {
	if in == nil {
		return nil
	}
	out := make(map[string][]string, len(in))
	for sid, values := range in {
		out[sid] = append([]string(nil), values...)
	}
	return out
}

func renderPhase2Markdown(resp phase2RunResponse) string {
	var b strings.Builder
	b.WriteString("# Phase 2 Transcript\n\n")
	_, _ = fmt.Fprintf(&b, "- Action: `%s`\n", resp.Action)
	_, _ = fmt.Fprintf(&b, "- Stream mode: `%s`\n", resp.StreamMode)
	if resp.Error != "" {
		_, _ = fmt.Fprintf(&b, "- Error: `%s`\n", resp.Error)
	}
	b.WriteString("\n## Checks\n\n")
	keys := make([]string, 0, len(resp.Checks))
	for key := range resp.Checks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		status := "FAIL"
		if resp.Checks[key] {
			status = "PASS"
		}
		_, _ = fmt.Fprintf(&b, "- %s: %s\n", key, status)
	}
	b.WriteString("\n## Message History\n\n")
	for _, record := range resp.MessageHistory {
		buf, _ := json.Marshal(record)
		_, _ = fmt.Fprintf(&b, "- `%s`\n", string(buf))
	}
	b.WriteString("\n## Per-Session Ordinals\n\n```json\n")
	ordinals, _ := json.MarshalIndent(resp.PerSessionOrdinals, "", "  ")
	b.Write(ordinals)
	b.WriteString("\n```\n")
	b.WriteString("\n## Trace\n\n")
	for _, entry := range resp.Trace {
		_, _ = fmt.Fprintf(&b, "%d. **%s** — %s\n", entry.Step, entry.Kind, entry.Message)
		if len(entry.Details) > 0 {
			buf, _ := json.Marshal(entry.Details)
			_, _ = fmt.Fprintf(&b, "   - details: `%s`\n", string(buf))
		}
	}
	return b.String()
}

func protoStructMap(msg proto.Message) map[string]any {
	if pb, ok := msg.(*structpb.Struct); ok && pb != nil {
		return cloneMap(pb.AsMap())
	}
	return map[string]any{}
}

func uniqueStrings(values ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
