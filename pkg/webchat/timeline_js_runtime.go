package webchat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dop251/goja"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/structpb"
)

// JSTimelineRuntime executes JavaScript reducers and handlers for SEM events.
//
// Reducers register via registerSemReducer(eventType, fn) and may return:
// - a single timeline entity object
// - an array of entity objects
// - { upserts: <entity|array>, consume: boolean }
//
// Handlers register via onSem(eventType, fn) and are side-effect observers.
// Use "*" to subscribe to all event types.
type JSTimelineRuntime struct {
	mu sync.Mutex

	vm *goja.Runtime

	reducers map[string][]goja.Callable
	handlers map[string][]goja.Callable
}

func NewJSTimelineRuntime() *JSTimelineRuntime {
	r := &JSTimelineRuntime{
		vm:       goja.New(),
		reducers: map[string][]goja.Callable{},
		handlers: map[string][]goja.Callable{},
	}
	r.installHostAPIs()
	return r
}

func (r *JSTimelineRuntime) installHostAPIs() {
	if r == nil || r.vm == nil {
		return
	}

	if err := r.vm.Set("registerSemReducer", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(r.vm.NewTypeError("registerSemReducer(eventType, fn) requires 2 arguments"))
		}
		eventType := strings.TrimSpace(call.Arguments[0].String())
		if eventType == "" {
			panic(r.vm.NewTypeError("registerSemReducer: eventType must be non-empty"))
		}
		fn, ok := goja.AssertFunction(call.Arguments[1])
		if !ok {
			panic(r.vm.NewTypeError("registerSemReducer: second argument must be a function"))
		}
		r.reducers[eventType] = append(r.reducers[eventType], fn)
		return goja.Undefined()
	}); err != nil {
		panic(err)
	}

	if err := r.vm.Set("onSem", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(r.vm.NewTypeError("onSem(eventType, fn) requires 2 arguments"))
		}
		eventType := strings.TrimSpace(call.Arguments[0].String())
		if eventType == "" {
			eventType = "*"
		}
		fn, ok := goja.AssertFunction(call.Arguments[1])
		if !ok {
			panic(r.vm.NewTypeError("onSem: second argument must be a function"))
		}
		r.handlers[eventType] = append(r.handlers[eventType], fn)
		return goja.Undefined()
	}); err != nil {
		panic(err)
	}
}

func (r *JSTimelineRuntime) LoadScriptFile(path string) error {
	if r == nil {
		return errors.New("js timeline runtime: nil runtime")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("js timeline runtime: empty script path")
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "js timeline runtime: read script %q", path)
	}
	return r.LoadScriptSource(path, string(blob))
}

func (r *JSTimelineRuntime) LoadScriptSource(name string, source string) error {
	if r == nil || r.vm == nil {
		return errors.New("js timeline runtime: runtime not initialized")
	}
	if strings.TrimSpace(name) == "" {
		name = "timeline-runtime.js"
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.vm.RunScript(name, source); err != nil {
		return errors.Wrapf(err, "js timeline runtime: run script %q", name)
	}
	return nil
}

func (r *JSTimelineRuntime) HandleSemEvent(ctx context.Context, p *TimelineProjector, ev TimelineSemEvent, now int64) (bool, error) {
	if r == nil || r.vm == nil || p == nil {
		return false, nil
	}

	eventPayload := r.buildEventPayload(ev, now)
	ctxPayload := map[string]any{
		"now_ms": now,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	reducers := append([]goja.Callable{}, r.reducers[ev.Type]...)
	reducers = append(reducers, r.reducers["*"]...)
	handlers := append([]goja.Callable{}, r.handlers[ev.Type]...)
	handlers = append(handlers, r.handlers["*"]...)

	consume := false
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		_, err := handler(goja.Undefined(), r.vm.ToValue(eventPayload), r.vm.ToValue(ctxPayload))
		if err != nil {
			log.Warn().Err(err).Str("event_type", ev.Type).Msg("js timeline handler threw; continuing")
		}
	}

	for _, reducer := range reducers {
		if reducer == nil {
			continue
		}
		ret, err := reducer(goja.Undefined(), r.vm.ToValue(eventPayload), r.vm.ToValue(ctxPayload))
		if err != nil {
			log.Warn().Err(err).Str("event_type", ev.Type).Msg("js timeline reducer threw; continuing")
			continue
		}
		rc, entities := r.decodeReducerReturn(ret.Export(), ev, now)
		if rc {
			consume = true
		}
		for _, entity := range entities {
			if entity == nil {
				continue
			}
			if err := p.Upsert(ctx, ev.Seq, entity); err != nil {
				log.Warn().Err(err).Str("event_type", ev.Type).Msg("js timeline reducer upsert failed; continuing")
			}
		}
	}

	return consume, nil
}

func (r *JSTimelineRuntime) buildEventPayload(ev TimelineSemEvent, now int64) map[string]any {
	payload := map[string]any{
		"type":      ev.Type,
		"id":        ev.ID,
		"seq":       ev.Seq,
		"stream_id": ev.StreamID,
		"now_ms":    now,
	}
	if len(ev.Data) > 0 {
		var data any
		if err := json.Unmarshal(ev.Data, &data); err == nil {
			payload["data"] = data
		}
	}
	return payload
}

func (r *JSTimelineRuntime) decodeReducerReturn(raw any, ev TimelineSemEvent, now int64) (bool, []*timelinepb.TimelineEntityV2) {
	if raw == nil {
		return false, nil
	}
	if b, ok := raw.(bool); ok {
		return b, nil
	}

	if arr, ok := toAnySlice(raw); ok {
		return false, r.decodeEntityArray(arr, ev, now)
	}
	m, ok := toAnyMap(raw)
	if !ok {
		return false, nil
	}

	consume := toBool(m["consume"])
	if upsertsRaw, ok := m["upserts"]; ok {
		if arr, ok := toAnySlice(upsertsRaw); ok {
			return consume, r.decodeEntityArray(arr, ev, now)
		}
		if upsertMap, ok := toAnyMap(upsertsRaw); ok {
			if entity := decodeTimelineEntity(upsertMap, ev, now); entity != nil {
				return consume, []*timelinepb.TimelineEntityV2{entity}
			}
			return consume, nil
		}
		return consume, nil
	}

	if entity := decodeTimelineEntity(m, ev, now); entity != nil {
		return consume, []*timelinepb.TimelineEntityV2{entity}
	}
	return consume, nil
}

func (r *JSTimelineRuntime) decodeEntityArray(arr []any, ev TimelineSemEvent, now int64) []*timelinepb.TimelineEntityV2 {
	entities := make([]*timelinepb.TimelineEntityV2, 0, len(arr))
	for _, v := range arr {
		m, ok := toAnyMap(v)
		if !ok {
			continue
		}
		if entity := decodeTimelineEntity(m, ev, now); entity != nil {
			entities = append(entities, entity)
		}
	}
	return entities
}

func decodeTimelineEntity(m map[string]any, ev TimelineSemEvent, now int64) *timelinepb.TimelineEntityV2 {
	id := firstNonEmptyString(m, "id")
	if id == "" {
		id = strings.TrimSpace(ev.ID)
	}
	if id == "" {
		return nil
	}
	kind := firstNonEmptyString(m, "kind")
	if kind == "" {
		kind = "js.timeline.entity"
	}
	createdAt := firstNonZeroInt64(m, "created_at_ms", "createdAtMs")
	if createdAt == 0 {
		createdAt = now
	}
	updatedAt := firstNonZeroInt64(m, "updated_at_ms", "updatedAtMs")
	if updatedAt == 0 {
		updatedAt = now
	}

	propsMap := map[string]any{}
	if rawProps, ok := m["props"]; ok {
		if p, ok := toAnyMap(rawProps); ok {
			propsMap = p
		}
	}
	props, err := structpb.NewStruct(propsMap)
	if err != nil {
		log.Warn().Err(err).Str("entity_id", id).Msg("js timeline reducer produced invalid props; using empty object")
		props, _ = structpb.NewStruct(map[string]any{})
	}

	meta := map[string]string{}
	if rawMeta, ok := m["meta"]; ok {
		if mm, ok := toAnyMap(rawMeta); ok {
			for k, v := range mm {
				ks := strings.TrimSpace(k)
				if ks == "" {
					continue
				}
				meta[ks] = strings.TrimSpace(fmt.Sprintf("%v", v))
			}
		}
	}

	return &timelinepb.TimelineEntityV2{
		Id:          id,
		Kind:        kind,
		CreatedAtMs: createdAt,
		UpdatedAtMs: updatedAt,
		Props:       props,
		Meta:        meta,
	}
}

func toAnyMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	m, ok := v.(map[string]any)
	return m, ok
}

func toAnySlice(v any) ([]any, bool) {
	if v == nil {
		return nil, false
	}
	a, ok := v.([]any)
	return a, ok
}

func toBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func firstNonEmptyString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if s, ok := value.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func firstNonZeroInt64(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			switch x := value.(type) {
			case int64:
				if x != 0 {
					return x
				}
			case int:
				if x != 0 {
					return int64(x)
				}
			case float64:
				if x != 0 {
					return int64(x)
				}
			case json.Number:
				if xi, err := x.Int64(); err == nil && xi != 0 {
					return xi
				}
			}
		}
	}
	return 0
}
