package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	chatapp "github.com/go-go-golems/pinocchio/pkg/evtstream/apps/chat"
	storememory "github.com/go-go-golems/pinocchio/pkg/evtstream/hydration/memory"
	wstransport "github.com/go-go-golems/pinocchio/pkg/evtstream/transport/ws"
)

type phase4RunRequest struct {
	Action    string `json:"action"`
	SessionID string `json:"sessionId"`
	Prompt    string `json:"prompt"`
}

type phase4RunResponse struct {
	Action      string                       `json:"action"`
	SessionID   string                       `json:"sessionId"`
	Prompt      string                       `json:"prompt"`
	Trace       []traceEntry                 `json:"trace"`
	Connections []wstransport.ConnectionInfo `json:"connections"`
	Snapshot    map[string]any               `json:"snapshot"`
	Checks      map[string]bool              `json:"checks"`
	Error       string                       `json:"error,omitempty"`
}

type phase4State struct {
	hub     *evtstream.Hub
	store   *storememory.Store
	ws      *wstransport.Server
	engine  *chatapp.Engine
	service *chatapp.Service
	trace   []traceEntry
	lastRun phase4RunResponse
}

func (e *labEnvironment) newPhase4State() (*phase4State, error) {
	state := &phase4State{}
	store := storememory.New()
	reg := evtstream.NewSchemaRegistry()
	if err := chatapp.RegisterSchemas(reg); err != nil {
		return nil, err
	}
	wsServer, err := wstransport.NewServer(hydrationSnapshotProvider{store: store}, wstransport.WithHooks(wstransport.Hooks{
		OnConnect: func(cid evtstream.ConnectionId) {
			e.appendPhase4Trace("transport", "phase 4 websocket connected", map[string]any{"connectionId": string(cid)})
		},
		OnDisconnect: func(cid evtstream.ConnectionId) {
			e.appendPhase4Trace("transport", "phase 4 websocket disconnected", map[string]any{"connectionId": string(cid)})
		},
		OnSubscribe: func(cid evtstream.ConnectionId, sid evtstream.SessionId, since uint64) {
			e.appendPhase4Trace("transport", "phase 4 subscribed", map[string]any{"connectionId": string(cid), "sessionId": string(sid), "sinceOrdinal": fmt.Sprintf("%d", since)})
		},
		OnSnapshotSent: func(cid evtstream.ConnectionId, sid evtstream.SessionId, snap evtstream.Snapshot) {
			e.appendPhase4Trace("transport", "phase 4 snapshot sent", map[string]any{"connectionId": string(cid), "sessionId": string(sid), "ordinal": fmt.Sprintf("%d", snap.Ordinal), "entityCount": len(snap.Entities)})
		},
		OnUIEventSent: func(cid evtstream.ConnectionId, sid evtstream.SessionId, ord uint64, event evtstream.UIEvent) {
			details := protoStructMap(event.Payload)
			details["connectionId"] = string(cid)
			details["sessionId"] = string(sid)
			details["ordinal"] = fmt.Sprintf("%d", ord)
			details["uiEvent"] = event.Name
			e.appendPhase4Trace("transport", "phase 4 ui event sent", details)
		},
	}))
	if err != nil {
		return nil, err
	}
	engine := chatapp.NewEngine(
		chatapp.WithChunkDelay(15*time.Millisecond),
		chatapp.WithHooks(chatapp.Hooks{
			OnBackendEvent: func(sessionID, eventName string, payload map[string]any) {
				details := cloneMap(payload)
				details["sessionId"] = sessionID
				details["eventName"] = eventName
				e.appendPhase4Trace("backend-event", "phase 4 backend event emitted", details)
			},
		}),
	)

	hub, err := evtstream.NewHub(
		evtstream.WithSchemaRegistry(reg),
		evtstream.WithHydrationStore(store),
		evtstream.WithUIFanout(wsServer),
	)
	if err != nil {
		return nil, err
	}
	if err := chatapp.Install(hub, engine); err != nil {
		return nil, err
	}
	service, err := chatapp.NewService(hub, engine)
	if err != nil {
		return nil, err
	}
	state.hub = hub
	state.store = store
	state.ws = wsServer
	state.engine = engine
	state.service = service
	return state, nil
}

func (e *labEnvironment) resetPhase4Only() error {
	newState, err := e.newPhase4State()
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.phase4 = newState
	e.mu.Unlock()
	return nil
}

func (e *labEnvironment) RunPhase4(ctx context.Context, in phase4RunRequest) (phase4RunResponse, error) {
	action := strings.TrimSpace(in.Action)
	if action == "" {
		action = "state"
	}
	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		sessionID = "chat-demo"
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		prompt = "Explain ordinals in plain language"
	}

	e.mu.Lock()
	state := e.phase4
	e.mu.Unlock()
	if state == nil {
		return phase4RunResponse{}, fmt.Errorf("phase 4 state is not initialized")
	}

	var err error
	switch action {
	case "send":
		e.appendPhase4Trace("control", "phase 4 send requested", map[string]any{"sessionId": sessionID, "prompt": prompt})
		err = state.service.SubmitPrompt(ctx, evtstream.SessionId(sessionID), prompt)
	case "stop":
		e.appendPhase4Trace("control", "phase 4 stop requested", map[string]any{"sessionId": sessionID})
		err = state.service.Stop(ctx, evtstream.SessionId(sessionID))
	case "await-idle":
		err = state.service.WaitIdle(ctx, evtstream.SessionId(sessionID))
	case "reset-phase4":
		err = e.resetPhase4Only()
	case "state":
		// no-op
	default:
		err = fmt.Errorf("unknown phase 4 action %q", action)
	}
	resp, buildErr := e.buildPhase4Response(action, sessionID, prompt)
	if buildErr != nil {
		return phase4RunResponse{}, buildErr
	}
	if err != nil {
		resp.Error = err.Error()
	}
	e.mu.Lock()
	if e.phase4 != nil {
		e.phase4.lastRun = clonePhase4RunResponse(resp)
	}
	e.mu.Unlock()
	return resp, nil
}

func (e *labEnvironment) buildPhase4Response(action, sessionID, prompt string) (phase4RunResponse, error) {
	e.mu.Lock()
	state := e.phase4
	if state == nil {
		e.mu.Unlock()
		return phase4RunResponse{}, fmt.Errorf("phase 4 state is not initialized")
	}
	trace := append([]traceEntry(nil), state.trace...)
	wsServer := state.ws
	service := state.service
	e.mu.Unlock()
	snap, err := service.Snapshot(context.Background(), evtstream.SessionId(sessionID))
	if err != nil {
		return phase4RunResponse{}, err
	}
	encoded := encodeSnapshot(snap)
	encoded["ordinal"] = fmt.Sprintf("%d", snap.Ordinal)
	checks := map[string]bool{
		"snapshotBeforeLive": phase4SnapshotBeforeLive(trace),
		"timelineMatchesUI":  phase4TimelineMatchesUI(trace, encoded),
		"hasChatEntity":      len(snap.Entities) > 0,
		"stopCoherent":       phase4StopCoherent(trace, encoded),
	}
	return phase4RunResponse{
		Action:      action,
		SessionID:   sessionID,
		Prompt:      prompt,
		Trace:       trace,
		Connections: wsServer.Connections(),
		Snapshot:    encoded,
		Checks:      checks,
	}, nil
}

func (e *labEnvironment) appendPhase4Trace(kind, message string, details map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.phase4 == nil {
		return
	}
	step := len(e.phase4.trace) + 1
	e.phase4.trace = append(e.phase4.trace, traceEntry{Step: step, Kind: kind, Message: message, Details: cloneMap(details)})
}

func phase4SnapshotBeforeLive(trace []traceEntry) bool {
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
		case "phase 4 snapshot sent":
			if firstSnapshot[key] == 0 {
				firstSnapshot[key] = entry.Step
			}
		case "phase 4 ui event sent":
			if firstLive[key] == 0 {
				firstLive[key] = entry.Step
			}
		}
	}
	for key, step := range firstLive {
		if firstSnapshot[key] == 0 || firstSnapshot[key] >= step {
			return false
		}
	}
	return true
}

func phase4TimelineMatchesUI(trace []traceEntry, snapshot map[string]any) bool {
	entities, _ := snapshot["entities"].([]map[string]any)
	if len(entities) == 0 {
		return false
	}
	payload, _ := entities[len(entities)-1]["payload"].(map[string]any)
	finalText := toString(payload["text"])
	lastUI := ""
	for _, entry := range trace {
		if entry.Message != "phase 4 ui event sent" {
			continue
		}
		if text := toString(entry.Details["text"]); text != "" {
			lastUI = text
		}
	}
	if lastUI == "" {
		return true
	}
	return lastUI == finalText
}

func phase4StopCoherent(trace []traceEntry, snapshot map[string]any) bool {
	stopRequested := false
	for _, entry := range trace {
		if entry.Message == "phase 4 stop requested" {
			stopRequested = true
			break
		}
	}
	if !stopRequested {
		return true
	}
	entities, _ := snapshot["entities"].([]map[string]any)
	if len(entities) == 0 {
		return false
	}
	payload, _ := entities[len(entities)-1]["payload"].(map[string]any)
	return toString(payload["status"]) == "stopped"
}

func clonePhase4RunResponse(in phase4RunResponse) phase4RunResponse {
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
