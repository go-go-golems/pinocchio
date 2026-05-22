package cmds

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	"github.com/stretchr/testify/require"
)

func TestRunWithOptionsStdinRPCMultiTurn(t *testing.T) {
	cmd := newRPCStdinTestCommand(t)
	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)

	stdin := strings.NewReader(strings.Join([]string{
		`{"version":1,"sessionId":"s1","requestId":"r1","submit":{"prompt":"first"}}`,
		`{"version":1,"sessionId":"s1","requestId":"r2","submit":{"prompt":"second"}}`,
		`{"version":1,"sessionId":"s1","requestId":"r3","shutdown":{}}`,
	}, "\n") + "\n")
	var out bytes.Buffer

	finalTurn, err := cmd.RunWithOptions(context.Background(),
		run.WithRunMode(run.RunModeRPCStdin),
		run.WithReader(stdin),
		run.WithWriter(&out),
		run.WithInferenceSettings(inferenceSettings),
		run.WithEngineFactory(countingEngineFactory{}),
	)
	require.NoError(t, err)
	require.NotNil(t, finalTurn)
	require.Contains(t, assistantTexts(finalTurn), "users=3") // seed prompt + first + second

	frames := parseRPCLines(t, out.String())
	var doneByRequest []string
	var sawR2Patch bool
	for _, frame := range frames {
		if done := frame.GetDone(); done != nil {
			doneByRequest = append(doneByRequest, frame.GetRequestId()+":"+done.GetStatus())
		}
		if ui := frame.GetUiEvent(); ui != nil && frame.GetRequestId() == "r2" && ui.GetName() == "ChatTextSegmentFinished" {
			sawR2Patch = true
		}
	}
	require.Contains(t, doneByRequest, "r1:ok")
	require.Contains(t, doneByRequest, "r2:ok")
	require.Contains(t, doneByRequest, "r3:shutdown")
	require.True(t, sawR2Patch, out.String())
}

func TestRunWithOptionsStdinRPCIsolatesSessionAccumulators(t *testing.T) {
	cmd := newRPCStdinTestCommand(t)
	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)

	stdin := strings.NewReader(strings.Join([]string{
		`{"version":1,"sessionId":"s1","requestId":"s1-r1","submit":{"prompt":"s1 first"}}`,
		`{"version":1,"sessionId":"s2","requestId":"s2-r1","submit":{"prompt":"s2 first"}}`,
		`{"version":1,"sessionId":"s1","requestId":"s1-r2","submit":{"prompt":"s1 second"}}`,
		`{"version":1,"sessionId":"s1","requestId":"shutdown","shutdown":{}}`,
	}, "\n") + "\n")
	var out bytes.Buffer

	finalTurn, err := cmd.RunWithOptions(context.Background(),
		run.WithRunMode(run.RunModeRPCStdin),
		run.WithReader(stdin),
		run.WithWriter(&out),
		run.WithInferenceSettings(inferenceSettings),
		run.WithEngineFactory(countingEngineFactory{}),
	)
	require.NoError(t, err)
	require.NotNil(t, finalTurn)
	require.Contains(t, assistantTexts(finalTurn), "users=3") // seed prompt + s1 first + s1 second

	done := map[string]string{}
	for _, frame := range parseRPCLines(t, out.String()) {
		if df := frame.GetDone(); df != nil {
			done[frame.GetRequestId()] = frame.GetSessionId() + ":" + df.GetStatus()
		}
	}
	require.Equal(t, "s1:ok", done["s1-r1"])
	require.Equal(t, "s2:ok", done["s2-r1"])
	require.Equal(t, "s1:ok", done["s1-r2"])
	require.Equal(t, "s1:shutdown", done["shutdown"])
}

func TestRunWithOptionsStdinRPCMalformedInputReportsError(t *testing.T) {
	cmd := newRPCStdinTestCommand(t)
	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)

	var out bytes.Buffer
	_, err = cmd.RunWithOptions(context.Background(),
		run.WithRunMode(run.RunModeRPCStdin),
		run.WithReader(strings.NewReader("not-json\n")),
		run.WithWriter(&out),
		run.WithInferenceSettings(inferenceSettings),
		run.WithEngineFactory(countingEngineFactory{}),
	)
	require.NoError(t, err)

	var sawInvalid bool
	for _, frame := range parseRPCLines(t, out.String()) {
		if ef := frame.GetError(); ef != nil && ef.GetCode() == "invalid_request_json" {
			sawInvalid = true
		}
	}
	require.True(t, sawInvalid, out.String())
}

func newRPCStdinTestCommand(t *testing.T) *PinocchioCommand {
	t.Helper()
	cmd, err := NewPinocchioCommand(&cmds.CommandDescription{
		Name:   "rpc-stdin-smoke",
		Short:  "rpc stdin smoke",
		Schema: schema.NewSchema(),
	}, WithPrompt("seed"))
	require.NoError(t, err)
	cmd.EngineFactory = countingEngineFactory{}
	return cmd
}

type countingEngineFactory struct{}

func (countingEngineFactory) CreateEngine(*settings.InferenceSettings) (engine.Engine, error) {
	return countingEngine{}, nil
}

func (countingEngineFactory) SupportedProviders() []string { return []string{"openai"} }
func (countingEngineFactory) DefaultProvider() string      { return "openai" }

type countingEngine struct{}

func (countingEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	out := t.Clone()
	count := 0
	for _, block := range out.Blocks {
		if block.Role == turns.RoleUser {
			count++
		}
	}
	turns.AppendBlock(out, turns.NewAssistantTextBlock(fmt.Sprintf("users=%d", count)))
	return out, nil
}

func assistantTexts(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	parts := []string{}
	for _, block := range t.Blocks {
		if block.Role == turns.RoleAssistant {
			if text, ok := block.Payload[turns.PayloadKeyText].(string); ok {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}
