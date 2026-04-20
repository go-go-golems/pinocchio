package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	storememory "github.com/go-go-golems/pinocchio/pkg/evtstream/hydration/memory"
	storesqlite "github.com/go-go-golems/pinocchio/pkg/evtstream/hydration/sqlite"
	wstransport "github.com/go-go-golems/pinocchio/pkg/evtstream/transport/ws"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	phase5CommandName = "PersistRecord"
	phase5EventName   = "RecordPersisted"
	phase5UIEventName = "RecordObserved"
	phase5EntityKind  = "RecordEntity"
	phase5BusTopic    = "evtstream.phase5"
)

type phase5RunRequest struct {
	Action    string `json:"action"`
	Mode      string `json:"mode"`
	SessionID string `json:"sessionId"`
	Text      string `json:"text"`
}

type phase5RunResponse struct {
	Action      string                       `json:"action"`
	Mode        string                       `json:"mode"`
	SessionID   string                       `json:"sessionId"`
	Text        string                       `json:"text"`
	Trace       []traceEntry                 `json:"trace"`
	Connections []wstransport.ConnectionInfo `json:"connections"`
	PreRestart  map[string]any               `json:"preRestart"`
	PostRestart map[string]any               `json:"postRestart"`
	Checks      map[string]bool              `json:"checks"`
	Error       string                       `json:"error,omitempty"`
}

type phase5Runtime struct {
	mode   string
	hub    *evtstream.Hub
	ws     *wstransport.Server
	store  evtstream.HydrationStore
	close  func() error
	cancel context.CancelFunc
	dbPath string
	reg    *evtstream.SchemaRegistry
}

type phase5State struct {
	runtime    *phase5Runtime
	trace      []traceEntry
	preRestart map[string]any
	lastRun    phase5RunResponse
	seq        int
}

func (e *labEnvironment) newPhase5State(mode string, existingDBPath string) (*phase5State, error) {
	state := &phase5State{}
	runtime, err := e.buildPhase5Runtime(mode, existingDBPath)
	if err != nil {
		return nil, err
	}
	state.runtime = runtime
	return state, nil
}

func (e *labEnvironment) buildPhase5Runtime(mode string, existingDBPath string) (*phase5Runtime, error) {
	mode = normalizePhase5Mode(mode)
	reg := evtstream.NewSchemaRegistry()
	for _, err := range []error{
		reg.RegisterCommand(phase5CommandName, &structpb.Struct{}),
		reg.RegisterEvent(phase5EventName, &structpb.Struct{}),
		reg.RegisterUIEvent(phase5UIEventName, &structpb.Struct{}),
		reg.RegisterTimelineEntity(phase5EntityKind, &structpb.Struct{}),
	} {
		if err != nil {
			return nil, err
		}
	}

	var (
		store   evtstream.HydrationStore
		closeFn func() error
		dbPath  string
	)
	if mode == "sql" {
		dbPath = existingDBPath
		if dbPath == "" {
			file, err := os.CreateTemp("", "evtstream-systemlab-phase5-*.sqlite")
			if err != nil {
				return nil, err
			}
			dbPath = file.Name()
			_ = file.Close()
		}
		dsn, err := storesqlite.FileDSN(dbPath)
		if err != nil {
			return nil, err
		}
		sqlStore, err := storesqlite.New(dsn, reg)
		if err != nil {
			return nil, err
		}
		store = sqlStore
		closeFn = sqlStore.Close
	} else {
		store = storememory.New()
		closeFn = func() error { return nil }
	}

	wsServer, err := wstransport.NewServer(hydrationSnapshotProvider{store: store}, wstransport.WithHooks(wstransport.Hooks{
		OnConnect: func(cid evtstream.ConnectionId) {
			e.appendPhase5Trace("transport", "phase 5 websocket connected", map[string]any{"connectionId": string(cid)})
		},
		OnDisconnect: func(cid evtstream.ConnectionId) {
			e.appendPhase5Trace("transport", "phase 5 websocket disconnected", map[string]any{"connectionId": string(cid)})
		},
		OnSubscribe: func(cid evtstream.ConnectionId, sid evtstream.SessionId, since uint64) {
			e.appendPhase5Trace("transport", "phase 5 subscribed", map[string]any{"connectionId": string(cid), "sessionId": string(sid), "sinceOrdinal": fmt.Sprintf("%d", since)})
		},
		OnSnapshotSent: func(cid evtstream.ConnectionId, sid evtstream.SessionId, snap evtstream.Snapshot) {
			e.appendPhase5Trace("transport", "phase 5 snapshot sent", map[string]any{"connectionId": string(cid), "sessionId": string(sid), "ordinal": fmt.Sprintf("%d", snap.Ordinal), "entityCount": len(snap.Entities)})
		},
		OnUIEventSent: func(cid evtstream.ConnectionId, sid evtstream.SessionId, ord uint64, event evtstream.UIEvent) {
			details := protoStructMap(event.Payload)
			details["connectionId"] = string(cid)
			details["sessionId"] = string(sid)
			details["ordinal"] = fmt.Sprintf("%d", ord)
			details["uiEvent"] = event.Name
			e.appendPhase5Trace("transport", "phase 5 ui event sent", details)
		},
	}))
	if err != nil {
		return nil, err
	}

	pubsub := gochannel.NewGoChannel(gochannel.Config{OutputChannelBuffer: 256}, watermill.NopLogger{})
	hub, err := evtstream.NewHub(
		evtstream.WithSchemaRegistry(reg),
		evtstream.WithHydrationStore(store),
		evtstream.WithUIFanout(wsServer),
		evtstream.WithEventBus(pubsub, pubsub, evtstream.WithBusTopic(phase5BusTopic)),
	)
	if err != nil {
		_ = closeFn()
		return nil, err
	}
	if err := hub.RegisterCommand(phase5CommandName, e.handlePhase5Command); err != nil {
		_ = closeFn()
		return nil, err
	}
	if err := hub.RegisterUIProjection(evtstream.UIProjectionFunc(e.phase5UIProjection)); err != nil {
		_ = closeFn()
		return nil, err
	}
	if err := hub.RegisterTimelineProjection(evtstream.TimelineProjectionFunc(e.phase5TimelineProjection)); err != nil {
		_ = closeFn()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := hub.Run(ctx); err != nil {
		cancel()
		_ = closeFn()
		return nil, err
	}
	return &phase5Runtime{mode: mode, hub: hub, ws: wsServer, store: store, close: closeFn, cancel: cancel, dbPath: dbPath, reg: reg}, nil
}

func (e *labEnvironment) resetPhase5Only(mode string) error {
	e.mu.Lock()
	old := e.phase5
	e.mu.Unlock()
	if old != nil && old.runtime != nil {
		_ = e.shutdownPhase5Runtime(old.runtime)
	}
	newState, err := e.newPhase5State(mode, "")
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.phase5 = newState
	e.mu.Unlock()
	return nil
}

func (e *labEnvironment) shutdownPhase5Runtime(runtime *phase5Runtime) error {
	if runtime == nil {
		return nil
	}
	if runtime.cancel != nil {
		runtime.cancel()
	}
	_ = runtime.hub.Shutdown(context.Background())
	if runtime.close != nil {
		return runtime.close()
	}
	return nil
}

func (e *labEnvironment) shutdownPhase5() error {
	e.mu.Lock()
	state := e.phase5
	e.phase5 = nil
	e.mu.Unlock()
	if state == nil {
		return nil
	}
	return e.shutdownPhase5Runtime(state.runtime)
}

func (e *labEnvironment) RunPhase5(ctx context.Context, in phase5RunRequest) (phase5RunResponse, error) {
	action := strings.TrimSpace(in.Action)
	if action == "" {
		action = "state"
	}
	mode := normalizePhase5Mode(in.Mode)
	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		sessionID = "persist-demo"
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		text = "persist this record"
	}

	e.mu.Lock()
	state := e.phase5
	e.mu.Unlock()
	if state == nil || state.runtime == nil || state.runtime.mode != mode {
		if err := e.resetPhase5Only(mode); err != nil {
			return phase5RunResponse{}, err
		}
		e.mu.Lock()
		state = e.phase5
		e.mu.Unlock()
	}

	var err error
	switch action {
	case "seed-session":
		e.appendPhase5Trace("control", "phase 5 seed requested", map[string]any{"mode": mode, "sessionId": sessionID, "text": text})
		payload, buildErr := structpb.NewStruct(map[string]any{"text": text})
		if buildErr != nil {
			return phase5RunResponse{}, buildErr
		}
		before, _ := state.runtime.hub.Cursor(ctx, evtstream.SessionId(sessionID))
		err = state.runtime.hub.Submit(ctx, evtstream.SessionId(sessionID), phase5CommandName, payload)
		if err == nil {
			err = e.waitForPhase5Cursor(sessionID, before+1)
		}
	case "restart-backend":
		pre, buildErr := e.phase5SnapshotFor(state.runtime, sessionID)
		if buildErr != nil {
			return phase5RunResponse{}, buildErr
		}
		e.mu.Lock()
		if e.phase5 != nil {
			e.phase5.preRestart = pre
		}
		e.mu.Unlock()
		dbPath := ""
		if mode == "sql" {
			dbPath = state.runtime.dbPath
		}
		if err = e.shutdownPhase5Runtime(state.runtime); err != nil {
			return phase5RunResponse{}, err
		}
		newState, buildErr := e.newPhase5State(mode, dbPath)
		if buildErr != nil {
			return phase5RunResponse{}, buildErr
		}
		e.mu.Lock()
		newState.preRestart = pre
		newState.trace = append([]traceEntry(nil), state.trace...)
		newState.seq = state.seq
		newState.trace = append(newState.trace, traceEntry{Step: len(newState.trace) + 1, Kind: "control", Message: "phase 5 backend restarted", Details: map[string]any{"mode": mode}})
		e.phase5 = newState
		e.mu.Unlock()
	case "reset-phase5":
		err = e.resetPhase5Only(mode)
	case "state":
		// no-op
	default:
		err = fmt.Errorf("unknown phase 5 action %q", action)
	}

	resp, buildErr := e.buildPhase5Response(action, mode, sessionID, text)
	if buildErr != nil {
		return phase5RunResponse{}, buildErr
	}
	if err != nil {
		resp.Error = err.Error()
	}
	e.mu.Lock()
	if e.phase5 != nil {
		e.phase5.lastRun = clonePhase5RunResponse(resp)
	}
	e.mu.Unlock()
	return resp, nil
}

func (e *labEnvironment) buildPhase5Response(action, mode, sessionID, text string) (phase5RunResponse, error) {
	e.mu.Lock()
	state := e.phase5
	if state == nil || state.runtime == nil {
		e.mu.Unlock()
		return phase5RunResponse{}, fmt.Errorf("phase 5 state is not initialized")
	}
	trace := append([]traceEntry(nil), state.trace...)
	preRestart := cloneMap(state.preRestart)
	runtime := state.runtime
	e.mu.Unlock()
	postRestart, err := e.phase5SnapshotFor(runtime, sessionID)
	if err != nil {
		return phase5RunResponse{}, err
	}
	checks := map[string]bool{
		"cursorPreserved":   phase5CursorPreserved(mode, preRestart, postRestart),
		"entitiesPreserved": phase5EntitiesPreserved(mode, preRestart, postRestart),
		"resumeWithoutGaps": phase5ResumeWithoutGaps(trace, sessionID),
		"modeIsSQL":         mode == "sql",
	}
	return phase5RunResponse{
		Action:      action,
		Mode:        mode,
		SessionID:   sessionID,
		Text:        text,
		Trace:       trace,
		Connections: runtime.ws.Connections(),
		PreRestart:  preRestart,
		PostRestart: postRestart,
		Checks:      checks,
	}, nil
}

func (e *labEnvironment) phase5SnapshotFor(runtime *phase5Runtime, sessionID string) (map[string]any, error) {
	snap, err := runtime.hub.Snapshot(context.Background(), evtstream.SessionId(sessionID))
	if err != nil {
		return nil, err
	}
	encoded := encodeSnapshot(snap)
	encoded["ordinal"] = fmt.Sprintf("%d", snap.Ordinal)
	return encoded, nil
}

func (e *labEnvironment) waitForPhase5Cursor(sessionID string, want uint64) error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		state := e.phase5
		e.mu.Unlock()
		if state == nil || state.runtime == nil {
			return fmt.Errorf("phase 5 runtime is not initialized")
		}
		cursor, err := state.runtime.hub.Cursor(context.Background(), evtstream.SessionId(sessionID))
		if err != nil {
			return err
		}
		if cursor >= want {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for phase 5 cursor %d", want)
}

func (e *labEnvironment) handlePhase5Command(ctx context.Context, cmd evtstream.Command, _ *evtstream.Session, pub evtstream.EventPublisher) error {
	payload := protoStructMap(cmd.Payload)
	text := strings.TrimSpace(toString(payload["text"]))
	if text == "" {
		text = fmt.Sprintf("record-%d", time.Now().UnixNano())
	}
	e.appendPhase5Trace("handler", "phase 5 handler invoked", map[string]any{"sessionId": string(cmd.SessionId), "text": text})
	pb, err := structpb.NewStruct(map[string]any{"text": text})
	if err != nil {
		return err
	}
	return pub.Publish(ctx, evtstream.Event{Name: phase5EventName, SessionId: cmd.SessionId, Payload: pb})
}

func (e *labEnvironment) phase5UIProjection(_ context.Context, ev evtstream.Event, _ *evtstream.Session, _ evtstream.TimelineView) ([]evtstream.UIEvent, error) {
	payload := protoStructMap(ev.Payload)
	payload["ordinal"] = fmt.Sprintf("%d", ev.Ordinal)
	pb, err := structpb.NewStruct(payload)
	if err != nil {
		return nil, err
	}
	e.appendPhase5Trace("ui-projection", "phase 5 ui projection emitted event", map[string]any{"sessionId": string(ev.SessionId), "ordinal": fmt.Sprintf("%d", ev.Ordinal), "text": payload["text"]})
	return []evtstream.UIEvent{{Name: phase5UIEventName, Payload: pb}}, nil
}

func (e *labEnvironment) phase5TimelineProjection(_ context.Context, ev evtstream.Event, _ *evtstream.Session, _ evtstream.TimelineView) ([]evtstream.TimelineEntity, error) {
	payload := protoStructMap(ev.Payload)
	id := fmt.Sprintf("record-%d", ev.Ordinal)
	pb, err := structpb.NewStruct(map[string]any{"text": payload["text"], "ordinal": fmt.Sprintf("%d", ev.Ordinal)})
	if err != nil {
		return nil, err
	}
	e.appendPhase5Trace("timeline-projection", "phase 5 timeline projection upserted entity", map[string]any{"sessionId": string(ev.SessionId), "ordinal": fmt.Sprintf("%d", ev.Ordinal), "entityId": id})
	return []evtstream.TimelineEntity{{Kind: phase5EntityKind, Id: id, Payload: pb}}, nil
}

func (e *labEnvironment) appendPhase5Trace(kind, message string, details map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.phase5 == nil {
		return
	}
	step := len(e.phase5.trace) + 1
	e.phase5.trace = append(e.phase5.trace, traceEntry{Step: step, Kind: kind, Message: message, Details: cloneMap(details)})
}

func normalizePhase5Mode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "sql") {
		return "sql"
	}
	return "memory"
}

func phase5CursorPreserved(mode string, pre, post map[string]any) bool {
	if pre == nil {
		return true
	}
	if mode != "sql" {
		return toString(post["ordinal"]) == "0"
	}
	return toString(pre["ordinal"]) == toString(post["ordinal"])
}

func phase5EntitiesPreserved(mode string, pre, post map[string]any) bool {
	if pre == nil {
		return true
	}
	preJSON := fmt.Sprintf("%v", pre["entities"])
	postJSON := fmt.Sprintf("%v", post["entities"])
	if mode != "sql" {
		return postJSON == "[]"
	}
	return preJSON == postJSON
}

func phase5ResumeWithoutGaps(trace []traceEntry, sessionID string) bool {
	prev := uint64(0)
	for _, entry := range trace {
		if entry.Message != "phase 5 ui projection emitted event" {
			continue
		}
		if toString(entry.Details["sessionId"]) != sessionID {
			continue
		}
		var current uint64
		_, _ = fmt.Sscanf(toString(entry.Details["ordinal"]), "%d", &current)
		if current == 0 {
			continue
		}
		if prev > 0 && current != prev+1 {
			return false
		}
		prev = current
	}
	return true
}

func clonePhase5RunResponse(in phase5RunResponse) phase5RunResponse {
	out := in
	out.Trace = make([]traceEntry, 0, len(in.Trace))
	for _, entry := range in.Trace {
		out.Trace = append(out.Trace, traceEntry{Step: entry.Step, Kind: entry.Kind, Message: entry.Message, Details: cloneMap(entry.Details)})
	}
	out.Connections = append([]wstransport.ConnectionInfo(nil), in.Connections...)
	out.PreRestart = cloneMap(in.PreRestart)
	out.PostRestart = cloneMap(in.PostRestart)
	out.Checks = cloneBoolMap(in.Checks)
	return out
}
