package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	storememory "github.com/go-go-golems/pinocchio/pkg/evtstream/hydration/memory"
	wstransport "github.com/go-go-golems/pinocchio/pkg/evtstream/transport/ws"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	phase3CommandName    = "HydrateReconnectSeed"
	phase3EventStarted   = "HydrateReconnectStarted"
	phase3EventChunk     = "HydrateReconnectChunk"
	phase3EventFinished  = "HydrateReconnectFinished"
	phase3UIStarted      = "HydrateReconnectMessageStarted"
	phase3UIAppended     = "HydrateReconnectMessageAppended"
	phase3UIFinished     = "HydrateReconnectMessageFinished"
	phase3TimelineEntity = "HydrateReconnectMessage"
)

type phase3RunRequest struct {
	Action    string `json:"action"`
	SessionID string `json:"sessionId"`
	Prompt    string `json:"prompt"`
}

type phase3RunResponse struct {
	Action      string                       `json:"action"`
	SessionID   string                       `json:"sessionId"`
	Prompt      string                       `json:"prompt"`
	Trace       []traceEntry                 `json:"trace"`
	Connections []wstransport.ConnectionInfo `json:"connections"`
	Snapshot    map[string]any               `json:"snapshot"`
	Checks      map[string]bool              `json:"checks"`
	Error       string                       `json:"error,omitempty"`
}

type phase3State struct {
	hub         *evtstream.Hub
	store       *storememory.Store
	ws          *wstransport.Server
	trace       []traceEntry
	sessionMeta map[string]map[string]any
	lastRun     phase3RunResponse
	messageSeq  int
}

func (e *labEnvironment) newPhase3State() (*phase3State, error) {
	state := &phase3State{sessionMeta: map[string]map[string]any{}}
	store := storememory.New()
	reg := evtstream.NewSchemaRegistry()
	for _, err := range []error{
		reg.RegisterCommand(phase3CommandName, &structpb.Struct{}),
		reg.RegisterEvent(phase3EventStarted, &structpb.Struct{}),
		reg.RegisterEvent(phase3EventChunk, &structpb.Struct{}),
		reg.RegisterEvent(phase3EventFinished, &structpb.Struct{}),
		reg.RegisterUIEvent(phase3UIStarted, &structpb.Struct{}),
		reg.RegisterUIEvent(phase3UIAppended, &structpb.Struct{}),
		reg.RegisterUIEvent(phase3UIFinished, &structpb.Struct{}),
		reg.RegisterTimelineEntity(phase3TimelineEntity, &structpb.Struct{}),
	} {
		if err != nil {
			return nil, err
		}
	}

	wsServer, err := wstransport.NewServer(hydrationSnapshotProvider{store: store}, wstransport.WithHooks(wstransport.Hooks{
		OnConnect: func(cid evtstream.ConnectionId) {
			e.appendPhase3Trace("transport", "phase 3 websocket connected", map[string]any{"connectionId": string(cid)})
		},
		OnDisconnect: func(cid evtstream.ConnectionId) {
			e.appendPhase3Trace("transport", "phase 3 websocket disconnected", map[string]any{"connectionId": string(cid)})
		},
		OnSubscribe: func(cid evtstream.ConnectionId, sid evtstream.SessionId, since uint64) {
			e.appendPhase3Trace("transport", "phase 3 subscribed", map[string]any{"connectionId": string(cid), "sessionId": string(sid), "sinceOrdinal": fmt.Sprintf("%d", since)})
		},
		OnUnsubscribe: func(cid evtstream.ConnectionId, sid evtstream.SessionId) {
			e.appendPhase3Trace("transport", "phase 3 unsubscribed", map[string]any{"connectionId": string(cid), "sessionId": string(sid)})
		},
		OnSnapshotSent: func(cid evtstream.ConnectionId, sid evtstream.SessionId, snap evtstream.Snapshot) {
			e.appendPhase3Trace("transport", "phase 3 snapshot sent", map[string]any{"connectionId": string(cid), "sessionId": string(sid), "ordinal": fmt.Sprintf("%d", snap.Ordinal), "entityCount": len(snap.Entities)})
		},
		OnUIEventSent: func(cid evtstream.ConnectionId, sid evtstream.SessionId, ord uint64, event evtstream.UIEvent) {
			e.appendPhase3Trace("transport", "phase 3 ui event sent", map[string]any{"connectionId": string(cid), "sessionId": string(sid), "ordinal": fmt.Sprintf("%d", ord), "uiEvent": event.Name})
		},
		OnClientFrame: func(cid evtstream.ConnectionId, frame map[string]any) {
			frame["connectionId"] = string(cid)
			e.appendPhase3Trace("client-frame", "phase 3 client frame received", frame)
		},
	}))
	if err != nil {
		return nil, err
	}

	hub, err := evtstream.NewHub(
		evtstream.WithSchemaRegistry(reg),
		evtstream.WithHydrationStore(store),
		evtstream.WithUIFanout(wsServer),
		evtstream.WithSessionMetadataFactory(func(_ context.Context, sid evtstream.SessionId) (any, error) {
			meta := map[string]any{"sessionId": string(sid), "createdBy": "evtstream-systemlab", "lab": "phase3"}
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.phase3 != nil {
				e.phase3.sessionMeta[string(sid)] = cloneMap(meta)
				e.phase3AppendTraceLocked("session", "phase 3 session created", meta)
			}
			return cloneMap(meta), nil
		}),
	)
	if err != nil {
		return nil, err
	}
	if err := hub.RegisterCommand(phase3CommandName, e.handlePhase3Command); err != nil {
		return nil, err
	}
	if err := hub.RegisterUIProjection(evtstream.UIProjectionFunc(e.phase3UIProjection)); err != nil {
		return nil, err
	}
	if err := hub.RegisterTimelineProjection(evtstream.TimelineProjectionFunc(e.phase3TimelineProjection)); err != nil {
		return nil, err
	}

	state.hub = hub
	state.store = store
	state.ws = wsServer
	return state, nil
}

func (e *labEnvironment) resetPhase3Only() error {
	newState, err := e.newPhase3State()
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.phase3 = newState
	e.mu.Unlock()
	return nil
}

func (e *labEnvironment) RunPhase3(ctx context.Context, in phase3RunRequest) (phase3RunResponse, error) {
	action := strings.TrimSpace(in.Action)
	if action == "" {
		action = "state"
	}
	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		sessionID = "reconnect-demo"
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		prompt = "watch reconnect preserve a coherent snapshot"
	}

	var err error
	switch action {
	case "seed-session":
		e.appendPhase3Trace("control", "phase 3 seed requested", map[string]any{"sessionId": sessionID, "prompt": prompt})
		payload, buildErr := structpb.NewStruct(map[string]any{"prompt": prompt})
		if buildErr != nil {
			return phase3RunResponse{}, buildErr
		}
		e.mu.Lock()
		state := e.phase3
		e.mu.Unlock()
		if state == nil {
			return phase3RunResponse{}, fmt.Errorf("phase 3 state is not initialized")
		}
		err = state.hub.Submit(ctx, evtstream.SessionId(sessionID), phase3CommandName, payload)
	case "reset-phase3":
		err = e.resetPhase3Only()
	case "state":
		// no-op
	default:
		err = fmt.Errorf("unknown phase 3 action %q", action)
	}

	resp, buildErr := e.buildPhase3Response(action, sessionID, prompt)
	if buildErr != nil {
		return phase3RunResponse{}, buildErr
	}
	if err != nil {
		resp.Error = err.Error()
	}
	e.mu.Lock()
	if e.phase3 != nil {
		e.phase3.lastRun = clonePhase3RunResponse(resp)
	}
	e.mu.Unlock()
	return resp, nil
}

func (e *labEnvironment) buildPhase3Response(action, sessionID, prompt string) (phase3RunResponse, error) {
	e.mu.Lock()
	state := e.phase3
	if state == nil {
		e.mu.Unlock()
		return phase3RunResponse{}, fmt.Errorf("phase 3 state is not initialized")
	}
	trace := append([]traceEntry(nil), state.trace...)
	wsServer := state.ws
	hub := state.hub
	e.mu.Unlock()

	snap, err := hub.Snapshot(context.Background(), evtstream.SessionId(sessionID))
	if err != nil {
		return phase3RunResponse{}, err
	}
	encoded := encodeSnapshot(snap)
	encoded["ordinal"] = fmt.Sprintf("%d", snap.Ordinal)
	connections := wsServer.Connections()
	resp := phase3RunResponse{
		Action:      action,
		SessionID:   sessionID,
		Prompt:      prompt,
		Trace:       trace,
		Connections: connections,
		Snapshot:    encoded,
		Checks: map[string]bool{
			"snapshotBeforeLive": phase3SnapshotBeforeLive(trace),
			"connectionsTracked": phase3ConnectionsTracked(connections),
			"sessionHydrated":    snap.Ordinal > 0 || len(snap.Entities) > 0,
			"clientConvergence":  phase3ClientConvergence(trace, string(snap.SessionId), snap.Ordinal),
		},
	}
	return resp, nil
}

func (e *labEnvironment) handlePhase3Command(ctx context.Context, cmd evtstream.Command, sess *evtstream.Session, pub evtstream.EventPublisher) error {
	sid := string(cmd.SessionId)
	payload := protoStructMap(cmd.Payload)
	prompt := strings.TrimSpace(toString(payload["prompt"]))
	if prompt == "" {
		prompt = "reconnect demo"
	}
	messageID := e.nextPhase3MessageID()
	e.appendPhase3Trace("handler", "phase 3 handler invoked", map[string]any{"sessionId": sid, "messageId": messageID, "hasSession": sess != nil})
	started, _ := structpb.NewStruct(map[string]any{"messageId": messageID, "prompt": prompt})
	if err := pub.Publish(ctx, evtstream.Event{Name: phase3EventStarted, SessionId: cmd.SessionId, Payload: started}); err != nil {
		return err
	}
	for _, chunk := range splitPrompt(prompt) {
		chunkPayload, _ := structpb.NewStruct(map[string]any{"messageId": messageID, "chunk": chunk})
		if err := pub.Publish(ctx, evtstream.Event{Name: phase3EventChunk, SessionId: cmd.SessionId, Payload: chunkPayload}); err != nil {
			return err
		}
	}
	finished, _ := structpb.NewStruct(map[string]any{"messageId": messageID, "text": prompt})
	return pub.Publish(ctx, evtstream.Event{Name: phase3EventFinished, SessionId: cmd.SessionId, Payload: finished})
}

func (e *labEnvironment) phase3UIProjection(_ context.Context, ev evtstream.Event, _ *evtstream.Session, _ evtstream.TimelineView) ([]evtstream.UIEvent, error) {
	payload := protoStructMap(ev.Payload)
	payload["ordinal"] = fmt.Sprintf("%d", ev.Ordinal)
	pb, err := structpb.NewStruct(payload)
	if err != nil {
		return nil, err
	}
	var name string
	switch ev.Name {
	case phase3EventStarted:
		name = phase3UIStarted
	case phase3EventChunk:
		name = phase3UIAppended
	case phase3EventFinished:
		name = phase3UIFinished
	default:
		return nil, nil
	}
	e.appendPhase3Trace("ui-projection", "phase 3 ui projection emitted event", map[string]any{"sessionId": string(ev.SessionId), "ordinal": fmt.Sprintf("%d", ev.Ordinal), "uiEvent": name, "sourceEvent": ev.Name})
	return []evtstream.UIEvent{{Name: name, Payload: pb}}, nil
}

func (e *labEnvironment) phase3TimelineProjection(_ context.Context, ev evtstream.Event, _ *evtstream.Session, view evtstream.TimelineView) ([]evtstream.TimelineEntity, error) {
	payload := protoStructMap(ev.Payload)
	messageID := toString(payload["messageId"])
	if messageID == "" {
		return nil, nil
	}
	var entity map[string]any
	switch ev.Name {
	case phase3EventStarted:
		entity = map[string]any{"messageId": messageID, "text": "", "status": "streaming"}
	case phase3EventChunk:
		entity = currentEntityMapForKind(view, phase3TimelineEntity, messageID)
		entity["messageId"] = messageID
		entity["text"] = fmt.Sprintf("%s%s", toString(entity["text"]), toString(payload["chunk"]))
		entity["status"] = "streaming"
	case phase3EventFinished:
		entity = currentEntityMapForKind(view, phase3TimelineEntity, messageID)
		entity["messageId"] = messageID
		entity["text"] = toString(payload["text"])
		entity["status"] = "finished"
	default:
		return nil, nil
	}
	pb, err := structpb.NewStruct(entity)
	if err != nil {
		return nil, err
	}
	e.appendPhase3Trace("timeline-projection", "phase 3 timeline projection upserted entity", map[string]any{"sessionId": string(ev.SessionId), "entityId": messageID, "ordinal": fmt.Sprintf("%d", ev.Ordinal), "sourceEvent": ev.Name})
	return []evtstream.TimelineEntity{{Kind: phase3TimelineEntity, Id: messageID, Payload: pb}}, nil
}

func (e *labEnvironment) appendPhase3Trace(kind, message string, details map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.phase3 == nil {
		return
	}
	e.phase3AppendTraceLocked(kind, message, details)
}

func (e *labEnvironment) phase3AppendTraceLocked(kind, message string, details map[string]any) {
	step := len(e.phase3.trace) + 1
	e.phase3.trace = append(e.phase3.trace, traceEntry{Step: step, Kind: kind, Message: message, Details: cloneMap(details)})
}

func (e *labEnvironment) nextPhase3MessageID() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.phase3 == nil {
		return "phase3-msg-1"
	}
	e.phase3.messageSeq++
	return fmt.Sprintf("phase3-msg-%d", e.phase3.messageSeq)
}

func phase3SnapshotBeforeLive(trace []traceEntry) bool {
	firstSnapshot := map[string]int{}
	firstLive := map[string]int{}
	for _, entry := range trace {
		cid := toString(entry.Details["connectionId"])
		sid := toString(entry.Details["sessionId"])
		if cid == "" || sid == "" {
			continue
		}
		key := cid + ":" + sid
		switch entry.Message {
		case "phase 3 snapshot sent":
			if firstSnapshot[key] == 0 {
				firstSnapshot[key] = entry.Step
			}
		case "phase 3 ui event sent":
			if firstLive[key] == 0 {
				firstLive[key] = entry.Step
			}
		}
	}
	for key, liveStep := range firstLive {
		if snapStep := firstSnapshot[key]; snapStep == 0 || snapStep >= liveStep {
			return false
		}
	}
	return true
}

func phase3ConnectionsTracked(infos []wstransport.ConnectionInfo) bool {
	for _, info := range infos {
		if info.ConnectionId == "" {
			return false
		}
	}
	return true
}

func phase3ClientConvergence(trace []traceEntry, sessionID string, ordinal uint64) bool {
	want := fmt.Sprintf("%d", ordinal)
	lastSeen := map[string]string{}
	for _, entry := range trace {
		if toString(entry.Details["sessionId"]) != sessionID {
			continue
		}
		cid := toString(entry.Details["connectionId"])
		if cid == "" {
			continue
		}
		if entry.Message != "phase 3 snapshot sent" && entry.Message != "phase 3 ui event sent" {
			continue
		}
		if ord := toString(entry.Details["ordinal"]); ord != "" {
			lastSeen[cid] = ord
		}
	}
	for _, got := range lastSeen {
		if got != want {
			return false
		}
	}
	return true
}

func clonePhase3RunResponse(in phase3RunResponse) phase3RunResponse {
	out := in
	out.Trace = make([]traceEntry, 0, len(in.Trace))
	for _, entry := range in.Trace {
		out.Trace = append(out.Trace, traceEntry{Step: entry.Step, Kind: entry.Kind, Message: entry.Message, Details: cloneMap(entry.Details)})
	}
	out.Connections = append([]wstransport.ConnectionInfo(nil), in.Connections...)
	out.Snapshot = cloneMap(in.Snapshot)
	out.Checks = cloneBoolMap(in.Checks)
	return out
}

func renderPhase3Markdown(resp phase3RunResponse) string {
	body, _ := json.MarshalIndent(resp, "", "  ")
	return "# Phase 3 Transcript\n\n```json\n" + string(body) + "\n```\n"
}

func currentEntityMapForKind(view evtstream.TimelineView, kind, id string) map[string]any {
	entity, ok := view.Get(kind, id)
	if !ok || entity.Payload == nil {
		return map[string]any{}
	}
	if pb, ok := entity.Payload.(*structpb.Struct); ok {
		return cloneMap(pb.AsMap())
	}
	return map[string]any{}
}
