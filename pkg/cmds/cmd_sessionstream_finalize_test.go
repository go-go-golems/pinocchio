package cmds

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	"github.com/stretchr/testify/require"
)

func TestDetermineRunModeKeepsDefaultOnBlockingStdoutPath(t *testing.T) {
	require.Equal(t, run.RunModeBlocking, determineRunMode(&cmdlayers.HelpersSettings{Output: "text"}))
	require.Equal(t, run.RunModeInteractive, determineRunMode(&cmdlayers.HelpersSettings{Interactive: true, Output: "text"}))
	require.Equal(t, run.RunModeBlocking, determineRunMode(&cmdlayers.HelpersSettings{Interactive: true, NonInteractive: true, Output: "text"}))
	require.Equal(t, run.RunModeChat, determineRunMode(&cmdlayers.HelpersSettings{StartInChat: true, Interactive: true, Output: "text"}))
	require.Equal(t, run.RunModeInteractive, determineRunMode(&cmdlayers.HelpersSettings{ForceInteractive: true, Interactive: true, Output: "text"}))
	require.Equal(t, run.RunModeRPCJSONL, determineRunMode(&cmdlayers.HelpersSettings{RPC: true, Output: "text"}))
	require.Equal(t, run.RunModeRPCJSONL, determineRunMode(&cmdlayers.HelpersSettings{Output: "jsonl"}))
}

func TestRunWithOptionsBlockingDebugEventsKeepsTextOnStdout(t *testing.T) {
	cmd := newRPCJSONLTestCommand(t, "blocking-debug-file", streamingEngineFactory{})
	debugPath := filepath.Join(t.TempDir(), "events", "debug.jsonl")
	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)

	var out bytes.Buffer
	_, err = cmd.RunWithOptions(context.Background(),
		run.WithRunMode(run.RunModeBlocking),
		run.WithWriter(&out),
		run.WithInferenceSettings(inferenceSettings),
		run.WithEngineFactory(cmd.EngineFactory),
		run.WithUISettings(&run.UISettings{Output: "text", DebugEventsJSONL: debugPath}),
	)
	require.NoError(t, err)
	require.Contains(t, out.String(), "streamed")
	require.NotContains(t, out.String(), "\"uiEvent\"")

	debugBytes, err := os.ReadFile(debugPath)
	require.NoError(t, err)
	debugFrames := parseRPCLines(t, string(debugBytes))
	require.NotEmpty(t, debugFrames)
	var sawPatch bool
	for _, frame := range debugFrames {
		if ui := frame.GetUiEvent(); ui != nil && ui.GetName() == "ChatTextPatch" {
			sawPatch = true
		}
	}
	require.True(t, sawPatch, string(debugBytes))
}

func TestRunWithOptionsRPCJSONLWritesDebugEventsJSONL(t *testing.T) {
	cmd := newRPCJSONLTestCommand(t, "rpc-jsonl-debug-file", streamingEngineFactory{})
	debugPath := filepath.Join(t.TempDir(), "events", "debug.jsonl")
	inferenceSettings, err := settings.NewInferenceSettings()
	require.NoError(t, err)

	var out bytes.Buffer
	_, err = cmd.RunWithOptions(context.Background(),
		run.WithRunMode(run.RunModeRPCJSONL),
		run.WithWriter(&out),
		run.WithInferenceSettings(inferenceSettings),
		run.WithEngineFactory(cmd.EngineFactory),
		run.WithUISettings(&run.UISettings{DebugEventsJSONL: debugPath}),
	)
	require.NoError(t, err)

	debugBytes, err := os.ReadFile(debugPath)
	require.NoError(t, err)
	debugFrames := parseRPCLines(t, string(debugBytes))
	require.NotEmpty(t, debugFrames)
	var sawPatch bool
	for _, frame := range debugFrames {
		if ui := frame.GetUiEvent(); ui != nil && ui.GetName() == "ChatTextPatch" {
			sawPatch = true
		}
	}
	require.True(t, sawPatch, string(debugBytes))

	stdoutFrames := parseRPCLines(t, out.String())
	require.NotEmpty(t, stdoutFrames)
	require.NotNil(t, stdoutFrames[0].GetHello())
}
