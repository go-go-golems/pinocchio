package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	storememory "github.com/go-go-golems/pinocchio/pkg/evtstream/hydration/memory"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	phase1CommandName    = "LabStart"
	phase1EventStarted   = "LabStarted"
	phase1EventChunk     = "LabChunk"
	phase1EventFinished  = "LabFinished"
	phase1UIStarted      = "LabMessageStarted"
	phase1UIAppended     = "LabMessageAppended"
	phase1UIFinished     = "LabMessageFinished"
	phase1TimelineEntity = "LabMessage"
)

type phase1RunRequest struct {
	SessionID   string `json:"sessionId"`
	Prompt      string `json:"prompt"`
	CommandName string `json:"commandName"`
}

type phase1RunResponse struct {
	SessionID string          `json:"sessionId"`
	Session   map[string]any  `json:"session"`
	Trace     []traceEntry    `json:"trace"`
	UIEvents  []namedPayload  `json:"uiEvents"`
	Snapshot  map[string]any  `json:"snapshot"`
	Checks    map[string]bool `json:"checks"`
	Error     string          `json:"error,omitempty"`
}

type namedPayload struct {
	Name    string         `json:"name"`
	Payload map[string]any `json:"payload"`
}

type traceEntry struct {
	Step    int            `json:"step"`
	Kind    string         `json:"kind"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type labEnvironment struct {
	mu          sync.Mutex
	hub         *evtstream.Hub
	store       *storememory.Store
	reg         *evtstream.SchemaRegistry
	sessionMeta map[string]map[string]any
	traces      map[string][]traceEntry
	uiEvents    map[string][]namedPayload
	lastRuns    map[string]phase1RunResponse
	messageSeq  int
	phase2      *phase2State
	phase3      *phase3State
	phase4      *phase4State
	phase5      *phase5State
}

func newLabEnvironment() (*labEnvironment, error) {
	env := &labEnvironment{}
	if err := env.Reset(); err != nil {
		return nil, err
	}
	return env, nil
}

func (e *labEnvironment) Reset() error {
	if err := e.shutdownPhase2(); err != nil {
		return err
	}
	if err := e.shutdownPhase5(); err != nil {
		return err
	}

	store := storememory.New()
	reg := evtstream.NewSchemaRegistry()
	for _, err := range []error{
		reg.RegisterCommand(phase1CommandName, &structpb.Struct{}),
		reg.RegisterEvent(phase1EventStarted, &structpb.Struct{}),
		reg.RegisterEvent(phase1EventChunk, &structpb.Struct{}),
		reg.RegisterEvent(phase1EventFinished, &structpb.Struct{}),
		reg.RegisterUIEvent(phase1UIStarted, &structpb.Struct{}),
		reg.RegisterUIEvent(phase1UIAppended, &structpb.Struct{}),
		reg.RegisterUIEvent(phase1UIFinished, &structpb.Struct{}),
		reg.RegisterTimelineEntity(phase1TimelineEntity, &structpb.Struct{}),
	} {
		if err != nil {
			return err
		}
	}

	hub, err := evtstream.NewHub(
		evtstream.WithSchemaRegistry(reg),
		evtstream.WithHydrationStore(store),
		evtstream.WithSessionMetadataFactory(func(_ context.Context, sid evtstream.SessionId) (any, error) {
			meta := map[string]any{
				"sessionId": string(sid),
				"createdBy": "evtstream-systemlab",
				"lab":       "phase1",
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			e.sessionMeta[string(sid)] = cloneMap(meta)
			e.appendTraceLocked(string(sid), "session", "session created", meta)
			return cloneMap(meta), nil
		}),
	)
	if err != nil {
		return err
	}
	if err := hub.RegisterCommand(phase1CommandName, e.handlePhase1Command); err != nil {
		return err
	}
	if err := hub.RegisterUIProjection(evtstream.UIProjectionFunc(e.phase1UIProjection)); err != nil {
		return err
	}
	if err := hub.RegisterTimelineProjection(evtstream.TimelineProjectionFunc(e.phase1TimelineProjection)); err != nil {
		return err
	}

	phase2, err := e.newPhase2State()
	if err != nil {
		return err
	}
	phase3, err := e.newPhase3State()
	if err != nil {
		return err
	}
	phase4, err := e.newPhase4State()
	if err != nil {
		return err
	}
	phase5, err := e.newPhase5State("memory", "")
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.sessionMeta = map[string]map[string]any{}
	e.traces = map[string][]traceEntry{}
	e.uiEvents = map[string][]namedPayload{}
	e.lastRuns = map[string]phase1RunResponse{}
	e.messageSeq = 0
	e.hub = hub
	e.store = store
	e.reg = reg
	e.phase2 = phase2
	e.phase3 = phase3
	e.phase4 = phase4
	e.phase5 = phase5
	e.mu.Unlock()

	return e.startPhase2()
}

func (e *labEnvironment) RunPhase1(ctx context.Context, in phase1RunRequest) (phase1RunResponse, error) {
	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		sessionID = "lab-session-1"
	}
	commandName := strings.TrimSpace(in.CommandName)
	if commandName == "" {
		commandName = phase1CommandName
	}

	e.mu.Lock()
	delete(e.traces, sessionID)
	delete(e.uiEvents, sessionID)
	e.appendTraceLocked(sessionID, "command", "submit command", map[string]any{
		"commandName": commandName,
		"prompt":      in.Prompt,
	})
	e.mu.Unlock()

	payload, err := structpb.NewStruct(map[string]any{"prompt": in.Prompt})
	if err != nil {
		return phase1RunResponse{}, err
	}
	err = e.hub.Submit(ctx, evtstream.SessionId(sessionID), commandName, payload)

	e.mu.Lock()
	trace := append([]traceEntry(nil), e.traces[sessionID]...)
	uiEvents := append([]namedPayload(nil), e.uiEvents[sessionID]...)
	metadata := cloneMap(e.sessionMeta[sessionID])
	e.mu.Unlock()

	snap, snapErr := e.hub.Snapshot(ctx, evtstream.SessionId(sessionID))
	if snapErr != nil {
		return phase1RunResponse{}, snapErr
	}
	resp := phase1RunResponse{
		SessionID: sessionID,
		Session: map[string]any{
			"id":       sessionID,
			"metadata": metadata,
		},
		Trace:    trace,
		UIEvents: uiEvents,
		Snapshot: encodeSnapshot(snap),
		Checks: map[string]bool{
			"sessionExists":    metadata != nil,
			"cursorAdvanced":   snap.Ordinal > 0,
			"timelineProduced": len(snap.Entities) > 0,
			"uiEventsProduced": len(uiEvents) > 0,
		},
	}
	if err != nil {
		resp.Error = err.Error()
	}

	e.mu.Lock()
	e.lastRuns[sessionID] = cloneRunResponse(resp)
	e.mu.Unlock()
	return resp, nil
}

func (e *labEnvironment) ExportPhase1(sessionID, format string) (string, string, []byte, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = "lab-session-1"
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "json"
	}

	e.mu.Lock()
	resp, ok := e.lastRuns[sessionID]
	e.mu.Unlock()
	if !ok {
		return "", "", nil, fmt.Errorf("no transcript available for session %q", sessionID)
	}

	safeSessionID := strings.ReplaceAll(sessionID, "/", "-")
	safeSessionID = strings.ReplaceAll(safeSessionID, " ", "-")
	base := fmt.Sprintf("phase1-transcript-%s", safeSessionID)
	switch format {
	case "json":
		body, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return "", "", nil, err
		}
		return base + ".json", "application/json", body, nil
	case "md", "markdown":
		return base + ".md", "text/markdown; charset=utf-8", []byte(renderPhase1Markdown(resp)), nil
	default:
		return "", "", nil, fmt.Errorf("unsupported export format %q", format)
	}
}

func (e *labEnvironment) handlePhase1Command(ctx context.Context, cmd evtstream.Command, sess *evtstream.Session, pub evtstream.EventPublisher) error {
	sid := string(cmd.SessionId)
	payload := cmd.Payload.(*structpb.Struct).AsMap()
	prompt, _ := payload["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = "hello from systemlab"
	}
	messageID := e.nextMessageID()
	e.appendTrace(sid, "handler", "handler invoked", map[string]any{
		"sessionId":  sid,
		"messageId":  messageID,
		"hasSession": sess != nil,
	})

	started, _ := structpb.NewStruct(map[string]any{"messageId": messageID, "prompt": prompt})
	if err := pub.Publish(ctx, evtstream.Event{Name: phase1EventStarted, SessionId: cmd.SessionId, Payload: started}); err != nil {
		return err
	}
	for _, chunk := range splitPrompt(prompt) {
		chunkPayload, _ := structpb.NewStruct(map[string]any{"messageId": messageID, "chunk": chunk})
		if err := pub.Publish(ctx, evtstream.Event{Name: phase1EventChunk, SessionId: cmd.SessionId, Payload: chunkPayload}); err != nil {
			return err
		}
	}
	finished, _ := structpb.NewStruct(map[string]any{"messageId": messageID, "text": prompt})
	if err := pub.Publish(ctx, evtstream.Event{Name: phase1EventFinished, SessionId: cmd.SessionId, Payload: finished}); err != nil {
		return err
	}
	return nil
}

func (e *labEnvironment) phase1UIProjection(_ context.Context, ev evtstream.Event, _ *evtstream.Session, _ evtstream.TimelineView) ([]evtstream.UIEvent, error) {
	payload := ev.Payload.(*structpb.Struct)
	var name string
	switch ev.Name {
	case phase1EventStarted:
		name = phase1UIStarted
	case phase1EventChunk:
		name = phase1UIAppended
	case phase1EventFinished:
		name = phase1UIFinished
	default:
		return nil, nil
	}
	e.recordUIEvent(string(ev.SessionId), name, payload.AsMap())
	e.appendTrace(string(ev.SessionId), "ui-projection", "ui projection emitted event", map[string]any{
		"sourceEvent": ev.Name,
		"uiEvent":     name,
		"ordinal":     ev.Ordinal,
	})
	return []evtstream.UIEvent{{Name: name, Payload: payload}}, nil
}

func (e *labEnvironment) phase1TimelineProjection(_ context.Context, ev evtstream.Event, _ *evtstream.Session, view evtstream.TimelineView) ([]evtstream.TimelineEntity, error) {
	payload := ev.Payload.(*structpb.Struct).AsMap()
	messageID, _ := payload["messageId"].(string)
	if messageID == "" {
		return nil, nil
	}
	var entity map[string]any
	switch ev.Name {
	case phase1EventStarted:
		entity = map[string]any{"messageId": messageID, "text": "", "status": "streaming"}
	case phase1EventChunk:
		chunk, _ := payload["chunk"].(string)
		entity = currentEntityMap(view, messageID)
		entity["messageId"] = messageID
		entity["text"] = fmt.Sprintf("%s%s", toString(entity["text"]), chunk)
		entity["status"] = "streaming"
	case phase1EventFinished:
		entity = currentEntityMap(view, messageID)
		entity["messageId"] = messageID
		if text, _ := payload["text"].(string); text != "" {
			entity["text"] = text
		}
		entity["status"] = "finished"
	default:
		return nil, nil
	}
	pb, err := structpb.NewStruct(entity)
	if err != nil {
		return nil, err
	}
	e.appendTrace(string(ev.SessionId), "timeline-projection", "timeline projection upserted entity", map[string]any{
		"sourceEvent": ev.Name,
		"entityId":    messageID,
		"ordinal":     ev.Ordinal,
	})
	return []evtstream.TimelineEntity{{Kind: phase1TimelineEntity, Id: messageID, Payload: pb}}, nil
}

func (e *labEnvironment) nextMessageID() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.messageSeq++
	return fmt.Sprintf("msg-%d", e.messageSeq)
}

func (e *labEnvironment) appendTrace(sessionID, kind, message string, details map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.appendTraceLocked(sessionID, kind, message, details)
}

func (e *labEnvironment) appendTraceLocked(sessionID, kind, message string, details map[string]any) {
	step := len(e.traces[sessionID]) + 1
	e.traces[sessionID] = append(e.traces[sessionID], traceEntry{Step: step, Kind: kind, Message: message, Details: cloneMap(details)})
}

func (e *labEnvironment) recordUIEvent(sessionID, name string, payload map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.uiEvents[sessionID] = append(e.uiEvents[sessionID], namedPayload{Name: name, Payload: cloneMap(payload)})
}

func splitPrompt(prompt string) []string {
	if len(prompt) <= 1 {
		return []string{prompt}
	}
	mid := len(prompt) / 2
	return []string{prompt[:mid], prompt[mid:]}
}

func currentEntityMap(view evtstream.TimelineView, id string) map[string]any {
	entity, ok := view.Get(phase1TimelineEntity, id)
	if !ok || entity.Payload == nil {
		return map[string]any{}
	}
	if pb, ok := entity.Payload.(*structpb.Struct); ok {
		return cloneMap(pb.AsMap())
	}
	return map[string]any{}
}

func encodeSnapshot(snap evtstream.Snapshot) map[string]any {
	entities := make([]map[string]any, 0, len(snap.Entities))
	for _, entity := range snap.Entities {
		payload := map[string]any{}
		if pb, ok := entity.Payload.(*structpb.Struct); ok && pb != nil {
			payload = cloneMap(pb.AsMap())
		}
		entities = append(entities, map[string]any{
			"kind":    entity.Kind,
			"id":      entity.Id,
			"payload": payload,
		})
	}
	return map[string]any{
		"sessionId": string(snap.SessionId),
		"ordinal":   snap.Ordinal,
		"entities":  entities,
	}
}

func cloneRunResponse(in phase1RunResponse) phase1RunResponse {
	out := in
	out.Session = cloneMap(in.Session)
	out.Trace = make([]traceEntry, 0, len(in.Trace))
	for _, entry := range in.Trace {
		out.Trace = append(out.Trace, traceEntry{Step: entry.Step, Kind: entry.Kind, Message: entry.Message, Details: cloneMap(entry.Details)})
	}
	out.UIEvents = make([]namedPayload, 0, len(in.UIEvents))
	for _, event := range in.UIEvents {
		out.UIEvents = append(out.UIEvents, namedPayload{Name: event.Name, Payload: cloneMap(event.Payload)})
	}
	out.Snapshot = cloneMap(in.Snapshot)
	out.Checks = cloneBoolMap(in.Checks)
	return out
}

func renderPhase1Markdown(resp phase1RunResponse) string {
	var b strings.Builder
	b.WriteString("# Phase 1 Transcript\n\n")
	_, _ = fmt.Fprintf(&b, "- Session ID: `%s`\n", resp.SessionID)
	if resp.Error != "" {
		_, _ = fmt.Fprintf(&b, "- Error: `%s`\n", resp.Error)
	}
	b.WriteString("\n## Checks\n\n")
	for name, ok := range resp.Checks {
		status := "FAIL"
		if ok {
			status = "PASS"
		}
		_, _ = fmt.Fprintf(&b, "- %s: %s\n", name, status)
	}
	b.WriteString("\n## Trace\n\n")
	for _, entry := range resp.Trace {
		_, _ = fmt.Fprintf(&b, "%d. **%s** — %s\n", entry.Step, entry.Kind, entry.Message)
		if len(entry.Details) > 0 {
			buf, _ := json.Marshal(entry.Details)
			_, _ = fmt.Fprintf(&b, "   - details: `%s`\n", string(buf))
		}
	}
	b.WriteString("\n## UI Events\n\n")
	for _, event := range resp.UIEvents {
		buf, _ := json.Marshal(event.Payload)
		_, _ = fmt.Fprintf(&b, "- `%s` — `%s`\n", event.Name, string(buf))
	}
	b.WriteString("\n## Snapshot\n\n```json\n")
	buf, _ := json.MarshalIndent(resp.Snapshot, "", "  ")
	b.Write(buf)
	b.WriteString("\n```\n")
	return b.String()
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
