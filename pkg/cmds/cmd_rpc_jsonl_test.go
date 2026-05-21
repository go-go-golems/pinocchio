package cmds

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	chatapprpcv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/rpc/v1"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestRunWithOptionsRPCJSONLEmitsProtoJSONLines(t *testing.T) {
	cmd := newRPCJSONLTestCommand(t, "rpc-jsonl-smoke", &recordingEngineFactory{})

	var out bytes.Buffer
	runRPCJSONLTestCommand(t, cmd, &out)

	frames := parseRPCLines(t, out.String())
	require.GreaterOrEqual(t, len(frames), 5, out.String())
	require.NotNil(t, frames[0].GetHello())
	require.Equal(t, "pinocchio.chatapp.rpc.v1", frames[0].GetHello().GetProtocol())
	require.NotNil(t, frames[len(frames)-1].GetDone())
	require.Equal(t, "ok", frames[len(frames)-1].GetDone().GetStatus())

	var sawAccepted, sawPatch, sawFinished, sawRunFinished, sawSnapshot bool
	for _, frame := range frames {
		if ui := frame.GetUiEvent(); ui != nil {
			switch ui.GetName() {
			case "ChatUserMessageAccepted":
				sawAccepted = true
			case "ChatTextPatch":
				sawPatch = true
			case "ChatTextSegmentFinished":
				sawFinished = true
			case "ChatRunFinished":
				sawRunFinished = true
			}
		}
		if frame.GetSnapshot() != nil {
			sawSnapshot = true
		}
	}
	require.True(t, sawAccepted, "expected user-message UI event in %s", out.String())
	require.True(t, sawFinished, "expected finished text UI event in %s", out.String())
	require.True(t, sawRunFinished, "expected run-finished UI event in %s", out.String())
	require.False(t, sawPatch, "recordingEngine is non-streaming and should exercise fallback instead")
	require.True(t, sawSnapshot, "expected snapshot frame in %s", out.String())
}

func TestRunWithOptionsRPCJSONLEmitsStreamingPatchEvents(t *testing.T) {
	cmd := newRPCJSONLTestCommand(t, "rpc-jsonl-streaming-smoke", streamingEngineFactory{})

	var out bytes.Buffer
	runRPCJSONLTestCommand(t, cmd, &out)

	var sawPatch, sawReasoningPatch, sawRunFinished bool
	for _, frame := range parseRPCLines(t, out.String()) {
		if ui := frame.GetUiEvent(); ui != nil {
			sawPatch = sawPatch || ui.GetName() == "ChatTextPatch"
			sawReasoningPatch = sawReasoningPatch || ui.GetName() == "ChatReasoningPatch"
			sawRunFinished = sawRunFinished || ui.GetName() == "ChatRunFinished"
		}
	}
	require.True(t, sawPatch, "expected streaming patch UI event in %s", out.String())
	require.True(t, sawReasoningPatch, "expected reasoning patch UI event in %s", out.String())
	require.True(t, sawRunFinished, "expected run-finished UI event in %s", out.String())
}

func TestRunWithOptionsRPCJSONLReturnsRuntimeFailureStatus(t *testing.T) {
	cmd := newRPCJSONLTestCommand(t, "rpc-jsonl-runtime-error", runtimeFailingEngineFactory{})

	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)

	var out bytes.Buffer
	_, err = cmd.RunWithOptions(context.Background(),
		run.WithRunMode(run.RunModeRPCJSONL),
		run.WithWriter(&out),
		run.WithInferenceSettings(inferenceSettings),
		run.WithEngineFactory(cmd.EngineFactory),
	)
	require.Error(t, err)

	var sawRunFailed, sawTerminalError bool
	var doneStatus string
	for _, frame := range parseRPCLines(t, out.String()) {
		if ui := frame.GetUiEvent(); ui != nil && ui.GetName() == "ChatRunFailed" {
			sawRunFailed = true
		}
		if ef := frame.GetError(); ef != nil {
			sawTerminalError = ef.GetTerminal() && strings.Contains(ef.GetMessage(), "runtime boom")
		}
		if done := frame.GetDone(); done != nil {
			doneStatus = done.GetStatus()
		}
	}
	require.True(t, sawRunFailed, out.String())
	require.True(t, sawTerminalError, out.String())
	require.Equal(t, "failed", doneStatus)
}

func TestRunWithOptionsRPCJSONLEmitsTerminalErrorFrame(t *testing.T) {
	cmd := newRPCJSONLTestCommand(t, "rpc-jsonl-error-smoke", failingEngineFactory{})

	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)

	var out bytes.Buffer
	_, err = cmd.RunWithOptions(context.Background(),
		run.WithRunMode(run.RunModeRPCJSONL),
		run.WithWriter(&out),
		run.WithInferenceSettings(inferenceSettings),
		run.WithEngineFactory(cmd.EngineFactory),
	)
	require.Error(t, err)

	var sawTerminalError bool
	for _, frame := range parseRPCLines(t, out.String()) {
		if ef := frame.GetError(); ef != nil {
			sawTerminalError = ef.GetTerminal() && strings.Contains(ef.GetMessage(), "failed to create engine")
		}
	}
	require.True(t, sawTerminalError, "expected terminal error frame in %s", out.String())
}

func newRPCJSONLTestCommand(t *testing.T, name string, engineFactory any) *PinocchioCommand {
	t.Helper()
	cmdSchema := schema.NewSchema()
	cmd, err := NewPinocchioCommand(&cmds.CommandDescription{
		Name:   name,
		Short:  "rpc jsonl smoke",
		Schema: cmdSchema,
	}, WithPrompt("say hello"))
	require.NoError(t, err)
	switch ef := engineFactory.(type) {
	case *recordingEngineFactory:
		cmd.EngineFactory = ef
	case streamingEngineFactory:
		cmd.EngineFactory = ef
	case failingEngineFactory:
		cmd.EngineFactory = ef
	case runtimeFailingEngineFactory:
		cmd.EngineFactory = ef
	default:
		t.Fatalf("unsupported engine factory %T", engineFactory)
	}
	return cmd
}

func runRPCJSONLTestCommand(t *testing.T, cmd *PinocchioCommand, out *bytes.Buffer) {
	t.Helper()
	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)
	_, err = cmd.RunWithOptions(context.Background(),
		run.WithRunMode(run.RunModeRPCJSONL),
		run.WithWriter(out),
		run.WithInferenceSettings(inferenceSettings),
		run.WithEngineFactory(cmd.EngineFactory),
	)
	require.NoError(t, err)
}

func parseRPCLines(t *testing.T, output string) []*chatapprpcv1.RpcLine {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	frames := make([]*chatapprpcv1.RpcLine, 0, len(lines))
	for _, line := range lines {
		var frame chatapprpcv1.RpcLine
		require.NoError(t, protojson.Unmarshal([]byte(line), &frame), line)
		require.Equal(t, uint32(1), frame.GetVersion())
		require.NotEmpty(t, frame.GetSessionId())
		frames = append(frames, &frame)
	}
	return frames
}

type streamingEngineFactory struct{}

func (streamingEngineFactory) CreateEngine(*settings.InferenceSettings) (engine.Engine, error) {
	return streamingEngine{}, nil
}

func (streamingEngineFactory) SupportedProviders() []string { return []string{"openai"} }
func (streamingEngineFactory) DefaultProvider() string      { return "openai" }

type streamingEngine struct{}

func (streamingEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	meta := gepevents.EventMetadata{SessionID: "sid"}
	reasoningCorr := gepevents.Correlation{SessionID: "sid", RunID: "run-1", ProviderCallID: "provider-call-1", SegmentID: "reasoning-1"}
	gepevents.PublishEventToContext(ctx, gepevents.NewReasoningSegmentStartedEvent(meta, reasoningCorr, "summary"))
	gepevents.PublishEventToContext(ctx, gepevents.NewReasoningDeltaEventWithSource(meta, reasoningCorr, "summary", "thinking", "thinking", 1))
	gepevents.PublishEventToContext(ctx, gepevents.NewReasoningSegmentFinishedEventWithSource(meta, reasoningCorr, "summary", "thinking", "stop"))

	corr := gepevents.Correlation{SessionID: "sid", RunID: "run-1", ProviderCallID: "provider-call-1", SegmentID: "segment-1"}
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentStartedEvent(meta, corr, "assistant"))
	gepevents.PublishEventToContext(ctx, gepevents.NewTextDeltaEvent(meta, corr, "streamed", "streamed", 1))
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentFinishedEvent(meta, corr, "streamed", "stop"))
	turns.AppendBlock(t, turns.NewAssistantTextBlock("streamed"))
	return t, nil
}

type failingEngineFactory struct{}

func (failingEngineFactory) CreateEngine(*settings.InferenceSettings) (engine.Engine, error) {
	return nil, errors.New("boom")
}

func (failingEngineFactory) SupportedProviders() []string { return []string{"openai"} }
func (failingEngineFactory) DefaultProvider() string      { return "openai" }

type runtimeFailingEngineFactory struct{}

func (runtimeFailingEngineFactory) CreateEngine(*settings.InferenceSettings) (engine.Engine, error) {
	return runtimeFailingEngine{}, nil
}

func (runtimeFailingEngineFactory) SupportedProviders() []string { return []string{"openai"} }
func (runtimeFailingEngineFactory) DefaultProvider() string      { return "openai" }

type runtimeFailingEngine struct{}

func (runtimeFailingEngine) RunInference(context.Context, *turns.Turn) (*turns.Turn, error) {
	return nil, errors.New("runtime boom")
}
