package events

import (
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
)

// EventPlanningStart marks the beginning of a planning phase for an agentic run.
type EventPlanningStart struct {
	gepevents.EventImpl
	RunID           string `json:"run_id"`
	Provider        string `json:"provider"`                     // e.g. "openai"
	PlannerModel    string `json:"planner_model"`                // model name / engine identifier
	MaxIterations   int    `json:"max_iterations"`               // maximum planning iterations allowed
	StartedAtUnixMs int64  `json:"started_at_unix_ms,omitempty"` // when planning began
}

func NewPlanningStart(metadata gepevents.EventMetadata, runID, provider, plannerModel string, maxIterations int, startedAtUnixMs int64) *EventPlanningStart {
	if startedAtUnixMs == 0 {
		startedAtUnixMs = time.Now().UnixMilli()
	}
	return &EventPlanningStart{
		EventImpl:       gepevents.EventImpl{Type_: gepevents.EventType("planning.start"), Metadata_: metadata},
		RunID:           runID,
		Provider:        provider,
		PlannerModel:    plannerModel,
		MaxIterations:   maxIterations,
		StartedAtUnixMs: startedAtUnixMs,
	}
}

var _ gepevents.Event = &EventPlanningStart{}

// EventPlanningIteration represents a single planning iteration.
type EventPlanningIteration struct {
	gepevents.EventImpl
	RunID           string                 `json:"run_id"`
	Provider        string                 `json:"provider,omitempty"`
	PlannerModel    string                 `json:"planner_model,omitempty"`
	MaxIterations   int                    `json:"max_iterations,omitempty"`
	Iteration       int                    `json:"iteration"`                 // 1-based
	Decision        string                 `json:"decision"`                  // "tool"|"reflect"|"respond"|...
	Action          string                 `json:"action,omitempty"`          // mirrors decision for protobuf payloads
	Strategy        string                 `json:"strategy"`                  // current planning strategy
	Progress        string                 `json:"progress"`                  // progress assessment
	Reasoning       string                 `json:"reasoning,omitempty"`       // optional reasoning
	ToolName        string                 `json:"tool_name,omitempty"`       // optional tool hint
	ReflectionText  string                 `json:"reflection_text,omitempty"` // optional reflection
	Extra           map[string]interface{} `json:"extra,omitempty"`           // unstructured extra
	EmittedAtUnixMs int64                  `json:"emitted_at_unix_ms,omitempty"`
}

func NewPlanningIteration(metadata gepevents.EventMetadata, runID string, iteration int, decision, strategy, progress string) *EventPlanningIteration {
	return &EventPlanningIteration{
		EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("planning.iteration"), Metadata_: metadata},
		RunID:     runID,
		Iteration: iteration,
		Decision:  decision,
		Action:    decision,
		Strategy:  strategy,
		Progress:  progress,
	}
}

var _ gepevents.Event = &EventPlanningIteration{}

// EventPlanningReflection represents the planner reflecting on progress.
type EventPlanningReflection struct {
	gepevents.EventImpl
	RunID           string  `json:"run_id"`
	Iteration       int     `json:"iteration"`
	ReflectionText  string  `json:"reflection_text"`
	ProgressScore   float64 `json:"progress_score,omitempty"`
	EmittedAtUnixMs int64   `json:"emitted_at_unix_ms,omitempty"`
}

func NewPlanningReflection(metadata gepevents.EventMetadata, runID string, iteration int, reflectionText string) *EventPlanningReflection {
	return &EventPlanningReflection{
		EventImpl:      gepevents.EventImpl{Type_: gepevents.EventType("planning.reflection"), Metadata_: metadata},
		RunID:          runID,
		Iteration:      iteration,
		ReflectionText: reflectionText,
	}
}

var _ gepevents.Event = &EventPlanningReflection{}

// EventPlanningComplete marks the end of the planning phase.
type EventPlanningComplete struct {
	gepevents.EventImpl
	RunID             string `json:"run_id"`
	Provider          string `json:"provider,omitempty"`
	PlannerModel      string `json:"planner_model,omitempty"`
	MaxIterations     int    `json:"max_iterations,omitempty"`
	TotalIterations   int    `json:"total_iterations"`
	FinalDecision     string `json:"final_decision"`
	StatusReason      string `json:"status_reason,omitempty"`
	FinalDirective    string `json:"final_directive,omitempty"`
	CompletedAtUnixMs int64  `json:"completed_at_unix_ms,omitempty"`
}

func NewPlanningComplete(metadata gepevents.EventMetadata, runID string, totalIterations int, finalDecision string) *EventPlanningComplete {
	return &EventPlanningComplete{
		EventImpl:         gepevents.EventImpl{Type_: gepevents.EventType("planning.complete"), Metadata_: metadata},
		RunID:             runID,
		TotalIterations:   totalIterations,
		FinalDecision:     finalDecision,
		CompletedAtUnixMs: time.Now().UnixMilli(),
	}
}

var _ gepevents.Event = &EventPlanningComplete{}

// EventExecutionStart marks the beginning of execution.
type EventExecutionStart struct {
	gepevents.EventImpl
	RunID           string `json:"run_id"`
	ExecutorModel   string `json:"executor_model,omitempty"`
	Directive       string `json:"directive,omitempty"`
	StartedAtUnixMs int64  `json:"started_at_unix_ms,omitempty"`
}

func NewExecutionStart(metadata gepevents.EventMetadata, runID, executorModel, directive string) *EventExecutionStart {
	return &EventExecutionStart{
		EventImpl:       gepevents.EventImpl{Type_: gepevents.EventType("execution.start"), Metadata_: metadata},
		RunID:           runID,
		ExecutorModel:   executorModel,
		Directive:       directive,
		StartedAtUnixMs: time.Now().UnixMilli(),
	}
}

var _ gepevents.Event = &EventExecutionStart{}

// EventExecutionComplete marks the end of execution.
type EventExecutionComplete struct {
	gepevents.EventImpl
	RunID             string `json:"run_id"`
	CompletedAtUnixMs int64  `json:"completed_at_unix_ms,omitempty"`
	Status            string `json:"status"` // completed|error
	ErrorMessage      string `json:"error_message,omitempty"`
	TokensUsed        int    `json:"tokens_used,omitempty"`
	ResponseLength    int    `json:"response_length,omitempty"`
}

func NewExecutionComplete(metadata gepevents.EventMetadata, runID, status, errorMessage string) *EventExecutionComplete {
	return &EventExecutionComplete{
		EventImpl:         gepevents.EventImpl{Type_: gepevents.EventType("execution.complete"), Metadata_: metadata},
		RunID:             runID,
		CompletedAtUnixMs: time.Now().UnixMilli(),
		Status:            status,
		ErrorMessage:      errorMessage,
	}
}

var _ gepevents.Event = &EventExecutionComplete{}

func init() {
	// Best-effort registration; ignore duplicate register errors in hot-reload/test scenarios.
	_ = gepevents.RegisterEventFactory("planning.start", func() gepevents.Event {
		return &EventPlanningStart{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("planning.start")}}
	})
	_ = gepevents.RegisterEventFactory("planning.iteration", func() gepevents.Event {
		return &EventPlanningIteration{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("planning.iteration")}}
	})
	_ = gepevents.RegisterEventFactory("planning.reflection", func() gepevents.Event {
		return &EventPlanningReflection{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("planning.reflection")}}
	})
	_ = gepevents.RegisterEventFactory("planning.complete", func() gepevents.Event {
		return &EventPlanningComplete{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("planning.complete")}}
	})
	_ = gepevents.RegisterEventFactory("execution.start", func() gepevents.Event {
		return &EventExecutionStart{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("execution.start")}}
	})
	_ = gepevents.RegisterEventFactory("execution.complete", func() gepevents.Event {
		return &EventExecutionComplete{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("execution.complete")}}
	})
}
