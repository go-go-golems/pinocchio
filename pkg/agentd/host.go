package agentd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
)

// Tool is an in-process tool supplied by the program that embeds the agent
// service. An agent offers it to the model when its YAML lists the tool's name
// under hostTools.
type Tool struct {
	Definition geptools.ToolDefinition
	// Run executes one call. A returned error is shown to the model as the
	// tool's error text (the call failed); it does not fail the run.
	Run func(ctx context.Context, s *Session, call ToolCall) (any, error)
}

// ToolCall is one model tool call handed to a host tool.
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
	Run   string
}

// TextHook supplies text for a session: page instructions at creation, or a
// turn's context block.
type TextHook func(ctx context.Context, s *Session) (string, error)

func (s *Server) hostTool(name string) *Tool {
	for i := range s.opts.Tools {
		if s.opts.Tools[i].Definition.Name == name {
			return &s.opts.Tools[i]
		}
	}
	return nil
}

// registerHostTools adds the agent's host tools to a turn's registry.
func (s *Server) registerHostTools(a *AgentConfig, reg geptools.ToolRegistry) error {
	for _, name := range a.HostTools {
		t := s.hostTool(name)
		if t == nil {
			return fmt.Errorf("agent %s lists host tool %q, which this server does not provide", a.Name, name)
		}
		if err := reg.RegisterTool(name, t.Definition); err != nil {
			return err
		}
	}
	return nil
}

// hostExecutor runs the agent's host tools in process and passes every other
// call to next (the browser bridge, then missingTool).
type hostExecutor struct {
	s    *Session
	next geptools.ToolExecutor
}

func (e *hostExecutor) ExecuteToolCall(ctx context.Context, call geptools.ToolCall, reg geptools.ToolRegistry) (*geptools.ToolResult, error) {
	t := e.s.srv.hostTool(call.Name)
	if t == nil || !e.s.Agent.hasHostTool(call.Name) {
		return e.next.ExecuteToolCall(ctx, call, reg)
	}
	run := e.s.runID()
	input := map[string]any{}
	if len(call.Arguments) > 0 {
		if err := json.Unmarshal(call.Arguments, &input); err != nil {
			input = map[string]any{}
		}
	}
	e.s.log.Append(EvToolRequested, run, map[string]any{"callId": call.ID, "name": call.Name, "input": input, "mode": "host"})
	res := &geptools.ToolResult{ID: call.ID}
	status := "success"
	value, err := t.Run(ctx, e.s, ToolCall{ID: call.ID, Name: call.Name, Input: input, Run: run})
	switch {
	case ctx.Err() != nil:
		status, res.Error = "cancelled", "cancelled"
	case err != nil:
		status, res.Error = "failed", err.Error()
	default:
		res.Result = value
	}
	e.s.log.Append(EvToolFinished, run, map[string]any{"callId": call.ID, "name": call.Name, "status": status, "error": res.Error, "result": res.Result})
	return res, nil
}

func (e *hostExecutor) ExecuteToolCalls(ctx context.Context, calls []geptools.ToolCall, reg geptools.ToolRegistry) ([]*geptools.ToolResult, error) {
	out := make([]*geptools.ToolResult, 0, len(calls))
	for _, c := range calls {
		r, err := e.ExecuteToolCall(ctx, c, reg)
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

// parseLabelFilter reads ?label=k:v (repeatable).
func parseLabelFilter(values []string) map[string]string {
	out := map[string]string{}
	for _, v := range values {
		if k, val, ok := strings.Cut(v, ":"); ok && k != "" {
			out[k] = val
		}
	}
	return out
}

func labelsMatch(have, want map[string]string) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}
