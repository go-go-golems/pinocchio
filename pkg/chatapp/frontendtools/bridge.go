package frontendtools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/invopop/jsonschema"
	zlog "github.com/rs/zerolog/log"
	"google.golang.org/protobuf/encoding/protojson"
)

type bridgeContextKey struct{}

var invalidProviderToolNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// ProviderToolName maps browser-facing tool names such as "cart.add" to the
// provider-safe identifier shape accepted by OpenAI Responses tools.
func ProviderToolName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "frontend_tool"
	}
	ret := invalidProviderToolNameChars.ReplaceAllString(trimmed, "_")
	ret = strings.Trim(ret, "_")
	if ret == "" {
		return "frontend_tool"
	}
	return ret
}

// BridgeContext carries the per-run sessionstream handles a Geppetto tool
// executor needs in order to turn a model tool call into a browser request.
type BridgeContext struct {
	SessionID sessionstream.SessionId
	MessageID string
	Publisher sessionstream.EventPublisher
}

func WithBridgeContext(ctx context.Context, bridge BridgeContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, bridgeContextKey{}, bridge)
}

func BridgeContextFromContext(ctx context.Context) (BridgeContext, bool) {
	if ctx == nil {
		return BridgeContext{}, false
	}
	bridge, ok := ctx.Value(bridgeContextKey{}).(BridgeContext)
	return bridge, ok
}

// BridgeExecutor adapts browser-registered frontend tools to Geppetto's
// ToolExecutor interface. Calls for tools present in the frontend manifest are
// routed through Manager.Request; all other calls delegate to Fallback.
type BridgeExecutor struct {
	Manager  *Manager
	Fallback geptools.ToolExecutor
}

func NewBridgeExecutor(manager *Manager, fallback geptools.ToolExecutor) *BridgeExecutor {
	if fallback == nil {
		fallback = geptools.NewDefaultToolExecutor(geptools.DefaultToolConfig())
	}
	return &BridgeExecutor{Manager: manager, Fallback: fallback}
}

func (e *BridgeExecutor) ExecuteToolCall(ctx context.Context, call geptools.ToolCall, registry geptools.ToolRegistry) (*geptools.ToolResult, error) {
	start := time.Now()
	bridge, ok := BridgeContextFromContext(ctx)
	frontendToolName := ""
	if ok && bridge.SessionID != "" && e != nil && e.Manager != nil {
		frontendToolName = e.Manager.ResolveProviderToolName(bridge.SessionID, call.Name)
	}
	if !ok || bridge.SessionID == "" || bridge.Publisher == nil || e == nil || e.Manager == nil || frontendToolName == "" {
		zlog.Debug().Str("tool", call.Name).Str("tool_call_id", call.ID).Bool("bridge_context", ok).Msg("delegating tool call to fallback executor")
		return e.fallback().ExecuteToolCall(ctx, call, registry)
	}

	zlog.Info().Str("session_id", string(bridge.SessionID)).Str("message_id", bridge.MessageID).Str("tool", frontendToolName).Str("provider_tool", call.Name).Str("tool_call_id", call.ID).Msg("routing tool call to browser frontend tool bridge")

	input := map[string]any{}
	if len(call.Arguments) > 0 {
		if err := json.Unmarshal(call.Arguments, &input); err != nil {
			return &geptools.ToolResult{ID: call.ID, Error: fmt.Sprintf("decode frontend tool arguments: %v", err), Duration: time.Since(start)}, nil
		}
	}

	desc, _ := e.Manager.Descriptor(bridge.SessionID, frontendToolName)
	mode := toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_AUTO
	if desc != nil && desc.GetMode() != toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_UNSPECIFIED {
		mode = desc.GetMode()
	}
	result, err := e.Manager.Request(ctx, bridge.SessionID, bridge.Publisher, Request{
		MessageID:  bridge.MessageID,
		ToolCallID: call.ID,
		ToolName:   frontendToolName,
		Input:      input,
		Mode:       mode,
	})
	if err != nil {
		zlog.Error().Err(err).Str("session_id", string(bridge.SessionID)).Str("tool", frontendToolName).Str("provider_tool", call.Name).Str("tool_call_id", call.ID).Msg("frontend tool bridge request failed")
		return &geptools.ToolResult{ID: call.ID, Error: err.Error(), Duration: time.Since(start)}, nil
	}
	out := map[string]any{}
	if result.GetResult() != nil {
		out = result.GetResult().AsMap()
	}
	status := result.GetStatus()
	if status == "" {
		status = "success"
	}
	toolResult := &geptools.ToolResult{ID: call.ID, Result: out, Duration: time.Since(start)}
	zlog.Info().Str("session_id", string(bridge.SessionID)).Str("tool", frontendToolName).Str("provider_tool", call.Name).Str("tool_call_id", call.ID).Str("status", status).Dur("duration", time.Since(start)).Msg("frontend tool bridge returned result")
	if status != "success" {
		if result.GetError() != "" {
			toolResult.Error = result.GetError()
		} else {
			toolResult.Error = fmt.Sprintf("frontend tool returned status %s", status)
		}
	}
	return toolResult, nil
}

func (e *BridgeExecutor) ExecuteToolCalls(ctx context.Context, calls []geptools.ToolCall, registry geptools.ToolRegistry) ([]*geptools.ToolResult, error) {
	out := make([]*geptools.ToolResult, 0, len(calls))
	for _, call := range calls {
		result, err := e.ExecuteToolCall(ctx, call, registry)
		if err != nil {
			return out, err
		}
		out = append(out, result)
	}
	return out, nil
}

func (e *BridgeExecutor) fallback() geptools.ToolExecutor {
	if e != nil && e.Fallback != nil {
		return e.Fallback
	}
	return geptools.NewDefaultToolExecutor(geptools.DefaultToolConfig())
}

// RegisterManifestTools adds the browser manifest for sid to a Geppetto tool
// registry so model providers can see frontend tools as ordinary tool
// definitions. Execution still goes through BridgeExecutor.
func (m *Manager) RegisterManifestTools(sid sessionstream.SessionId, registry geptools.ToolRegistry) error {
	if registry == nil {
		return fmt.Errorf("tool registry is nil")
	}
	m.mu.Lock()
	manifest := m.manifests[sid]
	m.mu.Unlock()
	if manifest == nil {
		zlog.Debug().Str("session_id", string(sid)).Msg("no frontend manifest available to register")
		return nil
	}
	providerNames := map[string]string{}
	for _, desc := range manifest.Tools {
		if desc == nil || !desc.GetAvailable() || desc.GetName() == "" {
			continue
		}
		providerName := ProviderToolName(desc.GetName())
		if existing, ok := providerNames[providerName]; ok && existing != desc.GetName() {
			return fmt.Errorf("frontend tool provider name collision: %q and %q both map to %q", existing, desc.GetName(), providerName)
		}
		providerNames[providerName] = desc.GetName()
		def := geptools.ToolDefinition{
			Name:        providerName,
			Description: frontendToolDescription(desc),
			Parameters:  descriptorSchema(desc),
			Tags:        []string{"frontend"},
		}
		if err := registry.RegisterTool(providerName, def); err != nil {
			return err
		}
		zlog.Debug().Str("session_id", string(sid)).Str("tool", desc.GetName()).Str("provider_tool", providerName).Msg("registered frontend manifest tool in geppetto registry")
	}
	return nil
}

func descriptorSchema(desc *toolv1.FrontendToolDescriptor) *jsonschema.Schema {
	if desc == nil || desc.GetInputSchema() == nil {
		return &jsonschema.Schema{Type: "object"}
	}
	b, err := protojson.Marshal(desc.GetInputSchema())
	if err != nil {
		return &jsonschema.Schema{Type: "object"}
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(b, &schema); err != nil {
		return &jsonschema.Schema{Type: "object"}
	}
	if schema.Type == "" && schema.Ref == "" {
		schema.Type = "object"
	}
	return &schema
}

func frontendToolDescription(desc *toolv1.FrontendToolDescriptor) string {
	if desc == nil {
		return ""
	}
	name := desc.GetName()
	description := strings.TrimSpace(desc.GetDescription())
	if name == "" || ProviderToolName(name) == name {
		return description
	}
	if description == "" {
		return fmt.Sprintf("Frontend browser tool %s.", name)
	}
	return fmt.Sprintf("%s\n\nFrontend browser tool name: %s.", description, name)
}

// ResolveProviderToolName returns the browser-facing frontend tool name for a
// provider tool call name. It accepts both raw manifest names and sanitized
// provider names so unit tests and non-OpenAI providers can still use raw names.
func (m *Manager) ResolveProviderToolName(sid sessionstream.SessionId, providerName string) string {
	if m == nil || providerName == "" {
		return ""
	}
	if m.HasAvailableTool(sid, providerName) {
		return providerName
	}
	m.mu.Lock()
	manifest := m.manifests[sid]
	m.mu.Unlock()
	if manifest == nil {
		return ""
	}
	for _, desc := range manifest.Tools {
		if desc == nil || !desc.GetAvailable() || desc.GetName() == "" {
			continue
		}
		if ProviderToolName(desc.GetName()) == providerName {
			return desc.GetName()
		}
	}
	return ""
}
