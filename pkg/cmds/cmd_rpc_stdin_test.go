package cmds

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

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

	stdin := delayedJSONLReader(t, 20*time.Millisecond,
		`{"version":1,"sessionId":"s1","requestId":"r1","submit":{"prompt":"first"}}`,
		`{"version":1,"sessionId":"s1","requestId":"r2","submit":{"prompt":"second"}}`,
		`{"version":1,"sessionId":"s1","requestId":"r3","shutdown":{}}`,
	)
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
	require.Contains(t, frames[0].GetHello().GetCapabilities(), "single-session")
	require.Contains(t, frames[0].GetHello().GetCapabilities(), "cancel")
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

func TestRunWithOptionsStdinRPCRejectsDifferentSession(t *testing.T) {
	cmd := newRPCStdinTestCommand(t)
	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)

	stdin := delayedJSONLReader(t, 20*time.Millisecond,
		`{"version":1,"sessionId":"s1","requestId":"s1-r1","submit":{"prompt":"s1 first"}}`,
		`{"version":1,"sessionId":"s2","requestId":"s2-r1","submit":{"prompt":"s2 first"}}`,
		`{"version":1,"sessionId":"s1","requestId":"shutdown","shutdown":{}}`,
	)
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
	require.Contains(t, assistantTexts(finalTurn), "users=2") // seed prompt + s1 first

	done := map[string]string{}
	var sawMismatch bool
	for _, frame := range parseRPCLines(t, out.String()) {
		if ef := frame.GetError(); ef != nil && ef.GetCode() == "session_mismatch" {
			sawMismatch = frame.GetRequestId() == "s2-r1"
		}
		if df := frame.GetDone(); df != nil {
			done[frame.GetRequestId()] = frame.GetSessionId() + ":" + df.GetStatus()
		}
	}
	require.True(t, sawMismatch, out.String())
	require.Equal(t, "s1:ok", done["s1-r1"])
	require.Equal(t, "s1:failed", done["s2-r1"])
	require.Equal(t, "s1:shutdown", done["shutdown"])
}

func TestRunWithOptionsStdinRPCRejectsSubmitWhileActive(t *testing.T) {
	cmd := newRPCStdinTestCommand(t)
	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)

	stdinReader, stdinWriter := io.Pipe()
	t.Cleanup(func() { _ = stdinReader.Close() })
	go func() {
		_, _ = fmt.Fprintln(stdinWriter, `{"version":1,"sessionId":"s1","requestId":"run","submit":{"prompt":"block until canceled"}}`)
		time.Sleep(50 * time.Millisecond)
		_, _ = fmt.Fprintln(stdinWriter, `{"version":1,"sessionId":"s1","requestId":"busy","submit":{"prompt":"should be rejected"}}`)
		_, _ = fmt.Fprintln(stdinWriter, `{"version":1,"sessionId":"s1","requestId":"cancel","cancel":{}}`)
		_, _ = fmt.Fprintln(stdinWriter, `{"version":1,"sessionId":"s1","requestId":"shutdown","shutdown":{}}`)
		_ = stdinWriter.Close()
	}()
	var out bytes.Buffer

	_, err = cmd.RunWithOptions(context.Background(),
		run.WithRunMode(run.RunModeRPCStdin),
		run.WithReader(stdinReader),
		run.WithWriter(&out),
		run.WithInferenceSettings(inferenceSettings),
		run.WithEngineFactory(blockingEngineFactory{}),
	)
	require.NoError(t, err)

	var sawBusy bool
	done := map[string]string{}
	for _, frame := range parseRPCLines(t, out.String()) {
		if ef := frame.GetError(); ef != nil && ef.GetCode() == "session_busy" {
			sawBusy = frame.GetRequestId() == "busy"
		}
		if df := frame.GetDone(); df != nil {
			done[frame.GetRequestId()] = df.GetStatus()
		}
	}
	require.True(t, sawBusy, out.String())
	require.Equal(t, "failed", done["busy"])
	require.Equal(t, "ok", done["cancel"])
	require.Equal(t, "stopped", done["run"])
	require.Equal(t, "shutdown", done["shutdown"])
}

func TestRunWithOptionsStdinRPCCancelWhileRunning(t *testing.T) {
	cmd := newRPCStdinTestCommand(t)
	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)

	stdinReader, stdinWriter := io.Pipe()
	t.Cleanup(func() { _ = stdinReader.Close() })
	go func() {
		_, _ = fmt.Fprintln(stdinWriter, `{"version":1,"sessionId":"s1","requestId":"run","submit":{"prompt":"block until canceled"}}`)
		time.Sleep(50 * time.Millisecond)
		_, _ = fmt.Fprintln(stdinWriter, `{"version":1,"sessionId":"s1","requestId":"cancel","cancel":{}}`)
		_, _ = fmt.Fprintln(stdinWriter, `{"version":1,"sessionId":"s1","requestId":"shutdown","shutdown":{}}`)
		_ = stdinWriter.Close()
	}()
	var out bytes.Buffer

	finalTurn, err := cmd.RunWithOptions(context.Background(),
		run.WithRunMode(run.RunModeRPCStdin),
		run.WithReader(stdinReader),
		run.WithWriter(&out),
		run.WithInferenceSettings(inferenceSettings),
		run.WithEngineFactory(blockingEngineFactory{}),
	)
	require.NoError(t, err)
	require.Nil(t, finalTurn)

	var sawStopped bool
	done := map[string]string{}
	for _, frame := range parseRPCLines(t, out.String()) {
		if ui := frame.GetUiEvent(); ui != nil {
			require.NotEqual(t, "cancel", frame.GetRequestId(), "control request id must not steal active submit UI frames")
			if ui.GetName() == "ChatRunStopped" && frame.GetRequestId() == "run" {
				sawStopped = true
			}
		}
		if df := frame.GetDone(); df != nil {
			done[frame.GetRequestId()] = df.GetStatus()
		}
	}
	require.True(t, sawStopped, out.String())
	require.Equal(t, "ok", done["cancel"])
	require.Equal(t, "stopped", done["run"])
	require.Equal(t, "shutdown", done["shutdown"])
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

func delayedJSONLReader(t *testing.T, delay time.Duration, lines ...string) io.Reader {
	t.Helper()
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = reader.Close() })
	go func() {
		for i, line := range lines {
			if i > 0 && delay > 0 {
				time.Sleep(delay)
			}
			_, _ = fmt.Fprintln(writer, line)
		}
		_ = writer.Close()
	}()
	return reader
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

type blockingEngineFactory struct{}

func (blockingEngineFactory) CreateEngine(*settings.InferenceSettings) (engine.Engine, error) {
	return blockingEngine{}, nil
}

func (blockingEngineFactory) SupportedProviders() []string { return []string{"openai"} }
func (blockingEngineFactory) DefaultProvider() string      { return "openai" }

type blockingEngine struct{}

func (blockingEngine) RunInference(ctx context.Context, _ *turns.Turn) (*turns.Turn, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

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
