package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/toolblocks"
	pinevents "github.com/go-go-golems/pinocchio/pkg/inference/events"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// LifecycleEngine wraps an engine.Engine with a planning→execution lifecycle.
//
// It performs one planning call per inference_id, emits planning.* + execution.* events,
// and injects the final directive via KeyDirective for the DirectiveMiddleware to apply.
//
// This mirrors the Moments planning widget contract:
// - a stable run_id correlation id
// - progressive iteration updates
// - execution start/complete milestones
type LifecycleEngine struct {
	inner engine.Engine
	cfg   Config

	providerLabel string
	modelLabel    string

	mu    sync.Mutex
	state map[string]*runState // keyed by inference_id
}

type runState struct {
	planned       bool
	execStarted   bool
	execCompleted bool
}

// NewLifecycleEngine constructs a LifecycleEngine wrapper around the provided inner engine.
//
// providerLabel/modelLabel are recorded in emitted planning events as descriptive metadata
// for the UI (e.g. "openai"/"gpt-4.1").
func NewLifecycleEngine(inner engine.Engine, cfg Config, providerLabel, modelLabel string) *LifecycleEngine {
	return &LifecycleEngine{
		inner:         inner,
		cfg:           cfg.Sanitized(),
		providerLabel: strings.TrimSpace(providerLabel),
		modelLabel:    strings.TrimSpace(modelLabel),
		state:         map[string]*runState{},
	}
}

func (e *LifecycleEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	if e == nil || e.inner == nil {
		return nil, errors.New("planning lifecycle engine is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if t == nil {
		t = &turns.Turn{}
	}

	runID := inferenceIDFromTurn(t)
	if runID == "" {
		runID = uuid.NewString()
		_ = turns.KeyTurnMetaInferenceID.Set(&t.Metadata, runID)
	}

	st := e.ensureState(runID)
	if !st.planned && e.cfg.Enabled {
		_ = e.planOnce(ctx, t, runID)
		st.planned = true
	}

	directive, ok, _ := KeyDirective.Get(t.Data)
	directive = strings.TrimSpace(directive)
	if ok && directive != "" && !st.execStarted {
		events.PublishEventToContext(ctx, pinevents.NewExecutionStart(e.eventMetadata(t), runID, e.modelLabel, directive))
		st.execStarted = true
	}

	updated, err := e.inner.RunInference(ctx, t)
	if err != nil && !st.execCompleted {
		events.PublishEventToContext(ctx, pinevents.NewExecutionComplete(e.eventMetadata(t), runID, "error", err.Error()))
		st.execCompleted = true
		e.deleteState(runID)
		return updated, err
	}

	// If the engine produced no pending tool calls, the toolloop will exit and we can finalize execution.
	if !st.execCompleted && !hasPendingTools(updated) {
		ev := pinevents.NewExecutionComplete(e.eventMetadata(t), runID, "completed", "")
		ev.ResponseLength = responseLength(updated)
		events.PublishEventToContext(ctx, ev)
		st.execCompleted = true
		e.deleteState(runID)
	}

	return updated, err
}

func (e *LifecycleEngine) ensureState(runID string) *runState {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.state == nil {
		e.state = map[string]*runState{}
	}
	existing := e.state[runID]
	if existing != nil {
		return existing
	}
	st := &runState{}
	e.state[runID] = st
	return st
}

func (e *LifecycleEngine) deleteState(runID string) {
	e.mu.Lock()
	delete(e.state, runID)
	e.mu.Unlock()
}

func (e *LifecycleEngine) planOnce(ctx context.Context, t *turns.Turn, runID string) error {
	md := e.eventMetadata(t)

	maxIters := e.cfg.MaxIterations
	if maxIters <= 0 {
		maxIters = DefaultConfig().MaxIterations
	}
	events.PublishEventToContext(ctx, pinevents.NewPlanningStart(md, runID, e.providerLabel, e.modelLabel, maxIters, time.Now().UnixMilli()))

	planTurn := t.Clone()
	setPlannerSystemPrompt(planTurn, e.cfg.Prompt)

	// Disable tools for the planner call (we want a JSON plan, not tool invocations).
	_ = engine.KeyToolConfig.Set(&planTurn.Data, engine.ToolConfig{Enabled: false})

	plannerCtx, cancel := plannerContext(ctx)
	defer cancel()
	planned, err := e.inner.RunInference(plannerCtx, planTurn)
	if err != nil {
		complete := pinevents.NewPlanningComplete(md, runID, 0, "error")
		complete.Provider = e.providerLabel
		complete.PlannerModel = e.modelLabel
		complete.MaxIterations = maxIters
		complete.StatusReason = "planner_inference_error"
		complete.FinalDirective = ""
		events.PublishEventToContext(ctx, complete)
		return err
	}

	raw, err := lastAssistantText(planned)
	if err != nil {
		complete := pinevents.NewPlanningComplete(md, runID, 0, "error")
		complete.Provider = e.providerLabel
		complete.PlannerModel = e.modelLabel
		complete.MaxIterations = maxIters
		complete.StatusReason = "planner_output_empty"
		events.PublishEventToContext(ctx, complete)
		return err
	}

	plan, err := parsePlanJSON(raw)
	if err != nil {
		complete := pinevents.NewPlanningComplete(md, runID, 0, "error")
		complete.Provider = e.providerLabel
		complete.PlannerModel = e.modelLabel
		complete.MaxIterations = maxIters
		complete.StatusReason = "planner_parse_error"
		complete.FinalDirective = ""
		events.PublishEventToContext(ctx, complete)
		return err
	}

	totalIterations := 0
	for _, it := range plan.Iterations {
		totalIterations++
		ev := pinevents.NewPlanningIteration(md, runID, it.IterationIndex, it.Action, it.Strategy, it.Progress)
		ev.Provider = e.providerLabel
		ev.PlannerModel = e.modelLabel
		ev.MaxIterations = maxIters
		ev.Reasoning = it.Reasoning
		ev.ToolName = it.ToolName
		ev.ReflectionText = it.ReflectionText
		ev.EmittedAtUnixMs = time.Now().UnixMilli()
		events.PublishEventToContext(ctx, ev)
	}

	finalDirective := strings.TrimSpace(plan.FinalDirective)
	if finalDirective != "" {
		_ = KeyDirective.Set(&t.Data, finalDirective)
	}

	complete := pinevents.NewPlanningComplete(md, runID, totalIterations, strings.TrimSpace(plan.FinalDecision))
	complete.Provider = e.providerLabel
	complete.PlannerModel = e.modelLabel
	complete.MaxIterations = maxIters
	complete.StatusReason = strings.TrimSpace(plan.StatusReason)
	complete.FinalDirective = finalDirective
	events.PublishEventToContext(ctx, complete)

	return nil
}

func (e *LifecycleEngine) eventMetadata(t *turns.Turn) events.EventMetadata {
	md := events.EventMetadata{ID: uuid.New()}
	if t == nil {
		return md
	}
	if sid, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata); err == nil && ok {
		md.SessionID = sid
	}
	if iid, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
		md.InferenceID = iid
	}
	md.TurnID = t.ID
	return md
}

func inferenceIDFromTurn(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	if iid, ok, err := turns.KeyTurnMetaInferenceID.Get(t.Metadata); err == nil && ok {
		return iid
	}
	return ""
}

func hasPendingTools(t *turns.Turn) bool {
	if t == nil {
		return false
	}
	return len(toolblocks.ExtractPendingToolCalls(t)) > 0
}

func lastAssistantText(t *turns.Turn) (string, error) {
	if t == nil {
		return "", ErrPlannerOutputEmpty
	}
	for i := len(t.Blocks) - 1; i >= 0; i-- {
		b := t.Blocks[i]
		if b.Role != turns.RoleAssistant {
			continue
		}
		if b.Payload == nil {
			continue
		}
		if s, ok := b.Payload[turns.PayloadKeyText].(string); ok && strings.TrimSpace(s) != "" {
			return s, nil
		}
	}
	return "", ErrPlannerOutputEmpty
}

func responseLength(t *turns.Turn) int {
	s, err := lastAssistantText(t)
	if err != nil {
		return 0
	}
	return len([]rune(s))
}

type planJSON struct {
	Iterations     []planIteration `json:"iterations"`
	FinalDecision  string          `json:"final_decision"`
	StatusReason   string          `json:"status_reason"`
	FinalDirective string          `json:"final_directive"`
}

type planIteration struct {
	IterationIndex int    `json:"iteration_index"`
	Action         string `json:"action"`
	Reasoning      string `json:"reasoning"`
	Strategy       string `json:"strategy"`
	Progress       string `json:"progress"`
	ToolName       string `json:"tool_name"`
	ReflectionText string `json:"reflection_text"`
}

func parsePlanJSON(raw string) (*planJSON, error) {
	s := strings.TrimSpace(raw)
	if s == "" || !strings.HasPrefix(s, "{") {
		return nil, fmt.Errorf("planner output is not JSON object")
	}
	var out planJSON
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, errors.Wrap(err, "unmarshal planner json")
	}
	if len(out.Iterations) == 0 {
		return nil, fmt.Errorf("planner output has no iterations")
	}
	for i := range out.Iterations {
		if out.Iterations[i].IterationIndex <= 0 {
			out.Iterations[i].IterationIndex = i + 1
		}
		if strings.TrimSpace(out.Iterations[i].Action) == "" {
			return nil, fmt.Errorf("planner iteration %d missing action", out.Iterations[i].IterationIndex)
		}
	}
	if strings.TrimSpace(out.FinalDecision) == "" {
		return nil, fmt.Errorf("planner output missing final_decision")
	}
	return &out, nil
}

func setPlannerSystemPrompt(t *turns.Turn, prompt string) {
	if t == nil {
		return
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return
	}
	for i, b := range t.Blocks {
		if b.Kind != turns.BlockKindSystem {
			continue
		}
		if t.Blocks[i].Payload == nil {
			t.Blocks[i].Payload = map[string]any{}
		}
		t.Blocks[i].Payload[turns.PayloadKeyText] = prompt
		// Prevent the normal systemprompt middleware from appending/rewriting anything:
		// for the planner call we want the planner prompt to dominate.
		_ = turns.KeyBlockMetaMiddleware.Set(&t.Blocks[i].Metadata, "systemprompt")
		return
	}
	// No system blocks: prepend one for the planner.
	b := turns.NewSystemTextBlock(prompt)
	_ = turns.KeyBlockMetaMiddleware.Set(&b.Metadata, "systemprompt")
	t.Blocks = append([]turns.Block{b}, t.Blocks...)
}

func plannerContext(parent context.Context) (context.Context, func()) {
	if parent == nil {
		ctx, cancel := context.WithCancel(context.Background())
		return ctx, cancel
	}

	base := context.Background()
	if dl, ok := parent.Deadline(); ok {
		ctx, cancel := context.WithDeadline(base, dl)
		go func() {
			select {
			case <-parent.Done():
				cancel()
			case <-ctx.Done():
			}
		}()
		return ctx, cancel
	}

	ctx, cancel := context.WithCancel(base)
	go func() {
		select {
		case <-parent.Done():
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}
