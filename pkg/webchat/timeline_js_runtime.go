package webchat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	gojengine "github.com/go-go-golems/go-go-goja/engine"
	"github.com/go-go-golems/go-go-goja/pkg/runtimeowner"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	TimelineJSModuleName  = "pinocchio"
	TimelineJSModuleAlias = "pnocchio"
)

// JSTimelineRuntime executes JavaScript reducers and handlers for SEM events.
//
// Reducers register via require("pinocchio").timeline.registerSemReducer(eventType, fn)
// and may return:
// - a single timeline entity object
// - an array of entity objects
// - { upserts: <entity|array>, consume: boolean }
//
// Handlers register via require("pinocchio").timeline.onSem(eventType, fn)
// and are side-effect observers.
// Use "*" to subscribe to all event types.
type JSTimelineRuntime struct {
	mu sync.RWMutex

	runtime *gojengine.Runtime
	vm      *goja.Runtime
	runner  runtimeowner.Runner

	closeOnce sync.Once

	reducers map[string][]goja.Callable
	handlers map[string][]goja.Callable
}

type JSTimelineRuntimeOptions struct {
	RequireOptions []require.Option
}

func NewJSTimelineRuntime() *JSTimelineRuntime {
	r, err := NewJSTimelineRuntimeWithOptions(JSTimelineRuntimeOptions{})
	if err != nil {
		panic(err)
	}
	return r
}

func NewJSTimelineRuntimeWithOptions(opts JSTimelineRuntimeOptions) (*JSTimelineRuntime, error) {
	r := &JSTimelineRuntime{
		reducers: map[string][]goja.Callable{},
		handlers: map[string][]goja.Callable{},
	}

	builderOpts := make([]gojengine.Option, 0, 1)
	if len(opts.RequireOptions) > 0 {
		builderOpts = append(builderOpts, gojengine.WithRequireOptions(opts.RequireOptions...))
	}
	builder := gojengine.NewBuilder(builderOpts...).WithModules(
		gojengine.NativeModuleSpec{
			ModuleID:   "timeline-runtime.pinocchio",
			ModuleName: TimelineJSModuleName,
			Loader:     r.pinocchioModuleLoader,
		},
		gojengine.NativeModuleSpec{
			ModuleID:   "timeline-runtime.pnocchio",
			ModuleName: TimelineJSModuleAlias,
			Loader:     r.pinocchioModuleLoader,
		},
	)
	factory, err := builder.Build()
	if err != nil {
		return nil, errors.Wrap(err, "js timeline runtime: build go-go-goja runtime factory")
	}
	ownedRuntime, err := factory.NewRuntime(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "js timeline runtime: create go-go-goja runtime")
	}

	r.runtime = ownedRuntime
	r.vm = ownedRuntime.VM
	r.runner = ownedRuntime.Owner
	return r, nil
}

func (r *JSTimelineRuntime) pinocchioModuleLoader(vm *goja.Runtime, moduleObj *goja.Object) {
	if r == nil || vm == nil || moduleObj == nil {
		return
	}
	exports := moduleObj.Get("exports")
	exportsObj := exports.ToObject(vm)
	timelineObj := vm.NewObject()
	if err := timelineObj.Set("registerSemReducer", func(call goja.FunctionCall) goja.Value {
		return r.registerSemReducerCall(vm, call)
	}); err != nil {
		panic(vm.NewGoError(err))
	}
	if err := timelineObj.Set("onSem", func(call goja.FunctionCall) goja.Value {
		return r.onSemCall(vm, call)
	}); err != nil {
		panic(vm.NewGoError(err))
	}
	if err := exportsObj.Set("timeline", timelineObj); err != nil {
		panic(vm.NewGoError(err))
	}
	// Top-level shortcuts keep script ergonomics flat while namespaced API remains canonical.
	if err := exportsObj.Set("registerSemReducer", timelineObj.Get("registerSemReducer")); err != nil {
		panic(vm.NewGoError(err))
	}
	if err := exportsObj.Set("onSem", timelineObj.Get("onSem")); err != nil {
		panic(vm.NewGoError(err))
	}
}

func (r *JSTimelineRuntime) registerSemReducerCall(vm *goja.Runtime, call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(vm.NewTypeError("registerSemReducer(eventType, fn) requires 2 arguments"))
	}
	eventType := strings.TrimSpace(call.Arguments[0].String())
	if eventType == "" {
		panic(vm.NewTypeError("registerSemReducer: eventType must be non-empty"))
	}
	fn, ok := goja.AssertFunction(call.Arguments[1])
	if !ok {
		panic(vm.NewTypeError("registerSemReducer: second argument must be a function"))
	}
	r.mu.Lock()
	r.reducers[eventType] = append(r.reducers[eventType], fn)
	r.mu.Unlock()
	return goja.Undefined()
}

func (r *JSTimelineRuntime) onSemCall(vm *goja.Runtime, call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(vm.NewTypeError("onSem(eventType, fn) requires 2 arguments"))
	}
	eventType := strings.TrimSpace(call.Arguments[0].String())
	if eventType == "" {
		eventType = "*"
	}
	fn, ok := goja.AssertFunction(call.Arguments[1])
	if !ok {
		panic(vm.NewTypeError("onSem: second argument must be a function"))
	}
	r.mu.Lock()
	r.handlers[eventType] = append(r.handlers[eventType], fn)
	r.mu.Unlock()
	return goja.Undefined()
}

func (r *JSTimelineRuntime) ensureRuntimeReady() error {
	if r == nil || r.vm == nil || r.runner == nil {
		return errors.New("js timeline runtime: runtime not initialized")
	}
	return nil
}

func (r *JSTimelineRuntime) ensurePinocchioModuleLoaded() error {
	if err := r.ensureRuntimeReady(); err != nil {
		return err
	}
	_, err := r.runner.Call(context.Background(), "timeline.ensurePinocchioModuleLoaded", func(_ context.Context, vm *goja.Runtime) (any, error) {
		_, runErr := vm.RunString(`require("` + TimelineJSModuleName + `")`)
		return nil, runErr
	})
	if err != nil {
		return errors.Wrap(err, "js timeline runtime: load pinocchio module")
	}
	return nil
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
	if err := r.ensureRuntimeReady(); err != nil {
		return err
	}
	if err := r.ensurePinocchioModuleLoaded(); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		name = "timeline-runtime.js"
	}
	if _, err := r.runner.Call(context.Background(), "timeline.LoadScriptSource", func(_ context.Context, vm *goja.Runtime) (any, error) {
		_, runErr := vm.RunScript(name, source)
		return nil, runErr
	}); err != nil {
		return errors.Wrapf(err, "js timeline runtime: run script %q", name)
	}
	return nil
}

func (r *JSTimelineRuntime) HandleSemEvent(ctx context.Context, p *TimelineProjector, ev TimelineSemEvent, now int64) (bool, error) {
	if r == nil || r.vm == nil || r.runner == nil || p == nil {
		return false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	eventPayload := r.buildEventPayload(ev, now)
	ctxPayload := map[string]any{
		"now_ms": now,
	}
	type dispatchResult struct {
		Consume  bool
		Entities []*timelinepb.TimelineEntityV2
	}
	rawResult, err := r.runner.Call(ctx, "timeline.HandleSemEvent", func(_ context.Context, vm *goja.Runtime) (any, error) {
		r.mu.RLock()
		reducers := append([]goja.Callable{}, r.reducers[ev.Type]...)
		reducers = append(reducers, r.reducers["*"]...)
		handlers := append([]goja.Callable{}, r.handlers[ev.Type]...)
		handlers = append(handlers, r.handlers["*"]...)
		r.mu.RUnlock()

		consume := false
		entities := []*timelinepb.TimelineEntityV2{}
		semEventValue := vm.ToValue(eventPayload)
		semCtxValue := vm.ToValue(ctxPayload)

		for _, handler := range handlers {
			if handler == nil {
				continue
			}
			_, runErr := handler(goja.Undefined(), semEventValue, semCtxValue)
			if runErr != nil {
				log.Warn().Err(runErr).Str("event_type", ev.Type).Msg("js timeline handler threw; continuing")
			}
		}

		for _, reducer := range reducers {
			if reducer == nil {
				continue
			}
			ret, runErr := reducer(goja.Undefined(), semEventValue, semCtxValue)
			if runErr != nil {
				log.Warn().Err(runErr).Str("event_type", ev.Type).Msg("js timeline reducer threw; continuing")
				continue
			}
			rc, reducedEntities := r.decodeReducerReturn(ret.Export(), ev, now)
			if rc {
				consume = true
			}
			for _, entity := range reducedEntities {
				if entity != nil {
					entities = append(entities, entity)
				}
			}
		}
		return dispatchResult{
			Consume:  consume,
			Entities: entities,
		}, nil
	})
	if err != nil {
		return false, errors.Wrap(err, "js timeline runtime: execute handlers/reducers")
	}
	result, ok := rawResult.(dispatchResult)
	if !ok {
		return false, errors.Errorf("js timeline runtime: unexpected reducer dispatch result type %T", rawResult)
	}
	for _, entity := range result.Entities {
		if entity == nil {
			continue
		}
		if err := p.Upsert(ctx, ev.Seq, entity); err != nil {
			log.Warn().Err(err).Str("event_type", ev.Type).Msg("js timeline reducer upsert failed; continuing")
		}
	}
	return result.Consume, nil
}

func (r *JSTimelineRuntime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var retErr error
	r.closeOnce.Do(func() {
		if r.runtime != nil {
			retErr = r.runtime.Close(ctx)
		}
		r.mu.Lock()
		r.reducers = map[string][]goja.Callable{}
		r.handlers = map[string][]goja.Callable{}
		r.mu.Unlock()
	})
	return retErr
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
