package mockruntime

import (
	"context"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

const (
	DefaultScenario = "parity_all"
	RuntimeKey      = "mock_parity"
)

type Options struct {
	Scenario   string
	ChunkDelay time.Duration
}

type Engine struct {
	scenario   string
	chunkDelay time.Duration
}

func NewEngine(opts Options) *Engine {
	scenario := opts.Scenario
	if scenario == "" {
		scenario = DefaultScenario
	}
	return &Engine{scenario: scenario, chunkDelay: opts.ChunkDelay}
}

func NewComposedRuntime(opts Options) infruntime.ComposedRuntime {
	return infruntime.ComposedRuntime{
		Engine:             NewEngine(opts),
		RuntimeKey:         RuntimeKey,
		RuntimeFingerprint: RuntimeKey + ":" + firstNonEmpty(opts.Scenario, DefaultScenario),
	}
}

func (e *Engine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	if t == nil {
		t = &turns.Turn{}
	} else {
		t = t.Clone()
	}

	meta := gepevents.EventMetadata{}
	providerCorr := gepevents.Correlation{RunID: "mock-run", ProviderCallID: "mock-provider-call"}
	publish(ctx, gepevents.NewProviderCallStartedEvent(meta, providerCorr))

	if err := e.publishReasoning(ctx, meta); err != nil {
		return t, err
	}
	if err := e.publishBackendTool(ctx, meta); err != nil {
		return t, err
	}
	publish(ctx, gepevents.NewAgentModeSwitchEvent(meta, "financial_analyst", "mock_reviewer", "- Mock profile selected\n- Deterministic parity event stream"))
	answer, err := e.publishText(ctx, meta)
	if err != nil {
		return t, err
	}

	duration := int64(25)
	publish(ctx, gepevents.NewProviderCallFinishedEvent(meta, providerCorr, "stop", "success", nil, &duration, true))
	turns.AppendBlock(t, turns.NewAssistantTextBlock(answer))
	return t, nil
}

func (e *Engine) publishReasoning(ctx context.Context, meta gepevents.EventMetadata) error {
	corr := gepevents.Correlation{RunID: "mock-run", ProviderCallID: "mock-provider-call", SegmentID: "mock-thinking-1"}
	publish(ctx, gepevents.NewReasoningSegmentStartedEvent(meta, corr, "mock"))
	chunks := []string{"Inspecting deterministic inputs. ", "Planning tool and widget coverage. ", "Ready to stream the mock answer."}
	accumulated := ""
	for i, chunk := range chunks {
		if err := e.wait(ctx); err != nil {
			return err
		}
		accumulated += chunk
		publish(ctx, gepevents.NewReasoningDeltaEventWithSource(meta, corr, "mock", chunk, accumulated, int64(i+1)))
	}
	publish(ctx, gepevents.NewReasoningSegmentFinishedEventWithSource(meta, corr, "mock", accumulated, "mock_complete"))
	return nil
}

func (e *Engine) publishBackendTool(ctx context.Context, meta gepevents.EventMetadata) error {
	corr := gepevents.Correlation{RunID: "mock-run", ProviderCallID: "mock-provider-call", ToolCallID: "mock-backend-tool-1"}
	input := `{"query":"mock parity","limit":3}`
	publish(ctx, gepevents.NewToolCallStartedEvent(meta, corr, "mock-backend-tool-1", "mock.search"))
	publish(ctx, gepevents.NewToolCallArgumentsDeltaEvent(meta, corr, "mock-backend-tool-1", input, input, 1))
	publish(ctx, gepevents.NewToolCallRequestedEvent(meta, corr, "mock-backend-tool-1", "mock.search", input))
	publish(ctx, gepevents.NewToolExecutionStartedEvent(meta, corr, "mock-backend-tool-1", "mock.search", input))
	if err := e.wait(ctx); err != nil {
		return err
	}
	publish(ctx, gepevents.NewToolResultReadyEvent(meta, corr, "mock-backend-tool-1", "mock.search", `{"ok":true,"hits":["reasoning","tool-call","chat-stream"]}`, "success"))
	publish(ctx, gepevents.NewToolCallFinishedEvent(meta, corr, "mock-backend-tool-1", "mock.search", "completed"))
	return nil
}

func (e *Engine) publishText(ctx context.Context, meta gepevents.EventMetadata) (string, error) {
	corr := gepevents.Correlation{RunID: "mock-run", ProviderCallID: "mock-provider-call", SegmentID: "mock-text-1"}
	publish(ctx, gepevents.NewTextSegmentStartedEvent(meta, corr, "assistant"))
	chunks := []string{"Mock parity run complete. ", "This deterministic profile emitted thinking, backend tool, special event, and chat text streams."}
	accumulated := ""
	for i, chunk := range chunks {
		if err := e.wait(ctx); err != nil {
			return accumulated, err
		}
		accumulated += chunk
		publish(ctx, gepevents.NewTextDeltaEvent(meta, corr, chunk, accumulated, int64(i+1)))
	}
	publish(ctx, gepevents.NewTextSegmentFinishedEvent(meta, corr, accumulated, "mock_complete"))
	return accumulated, nil
}

func (e *Engine) wait(ctx context.Context) error {
	if e.chunkDelay <= 0 {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(e.chunkDelay):
		return nil
	}
}

func publish(ctx context.Context, event gepevents.Event) {
	gepevents.PublishEventToContext(ctx, event)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
