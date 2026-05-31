package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/types/known/structpb"
)

type frontendToolManifestRequest struct {
	Revision uint64                      `json:"revision"`
	Tools    []frontendToolManifestEntry `json:"tools"`
}

type frontendToolManifestEntry struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Mode        string         `json:"mode"`
	InputSchema map[string]any `json:"inputSchema"`
	Available   bool           `json:"available"`
}

type frontendToolManifestResponse struct {
	Accepted bool   `json:"accepted"`
	Revision uint64 `json:"revision"`
}

type frontendToolResultRequest struct {
	ToolCallID string         `json:"toolCallId"`
	ToolName   string         `json:"toolName"`
	Status     string         `json:"status"`
	Result     map[string]any `json:"result"`
	Error      string         `json:"error"`
}

type frontendToolResultResponse struct {
	Accepted bool   `json:"accepted"`
	Status   string `json:"status"`
}

func parseWebChatSessionPath(path string) (string, string, bool) {
	const prefix = "/api/chat/sessions/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if rest == "" {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 1 {
		return parts[0], "", true
	}
	if parts[0] == "" {
		return "", "", false
	}
	return parts[0], strings.Join(parts[1:], "/"), true
}

func (s *Server) handleFrontendToolManifest(w http.ResponseWriter, r *http.Request, sid sessionstream.SessionId) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var in frontendToolManifestRequest
	if err := serverkit.DecodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad request"})
		return
	}
	descriptors := make([]*toolv1.FrontendToolDescriptor, 0, len(in.Tools))
	for _, tool := range in.Tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		inputSchema, err := structpb.NewStruct(tool.InputSchema)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("bad input schema for %s", name)})
			return
		}
		descriptors = append(descriptors, &toolv1.FrontendToolDescriptor{
			Name:        name,
			Description: tool.Description,
			InputSchema: inputSchema,
			Mode:        frontendToolMode(tool.Mode),
			Available:   tool.Available,
		})
	}
	if err := s.service.SubmitCommand(r.Context(), sid, frontendtools.CommandManifest, &toolv1.FrontendToolManifestCommand{
		Tools:    descriptors,
		Revision: in.Revision,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, frontendToolManifestResponse{Accepted: true, Revision: in.Revision})
}

func frontendToolMode(mode string) toolv1.ToolExecutionMode {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "frontend", "frontend_auto", "auto":
		return toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_AUTO
	case "human", "frontend_human":
		return toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_HUMAN
	case "backend":
		return toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_BACKEND
	default:
		return toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_UNSPECIFIED
	}
}

func (s *Server) handleFrontendToolResult(w http.ResponseWriter, r *http.Request, sid sessionstream.SessionId) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var in frontendToolResultRequest
	if err := serverkit.DecodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad request"})
		return
	}
	in.ToolCallID = strings.TrimSpace(in.ToolCallID)
	in.ToolName = strings.TrimSpace(in.ToolName)
	in.Status = strings.TrimSpace(in.Status)
	if in.ToolCallID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing toolCallId"})
		return
	}
	if in.ToolName == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing toolName"})
		return
	}
	if in.Status == "" {
		in.Status = "success"
	}
	if in.Result == nil {
		in.Result = map[string]any{}
	}
	if err := s.publishFrontendToolResult(r.Context(), sid, in); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, frontendToolResultResponse{Accepted: true, Status: in.Status})
}

func (s *Server) publishFrontendToolResult(ctx context.Context, sid sessionstream.SessionId, result frontendToolResultRequest) error {
	if s == nil || s.service == nil {
		return fmt.Errorf("server is not initialized")
	}
	resultPayload, err := structpb.NewStruct(result.Result)
	if err != nil {
		return fmt.Errorf("encode frontend tool result: %w", err)
	}
	return s.service.SubmitCommand(ctx, sid, frontendtools.CommandResult, &toolv1.FrontendToolResultCommand{
		ToolCallId: result.ToolCallID,
		ToolName:   result.ToolName,
		Result:     resultPayload,
		Status:     result.Status,
		Error:      result.Error,
	})
}
