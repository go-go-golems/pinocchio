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

func isCapabilitiesShowcasePrompt(prompt string) bool {
	p := strings.ToLower(strings.TrimSpace(prompt))
	return strings.Contains(p, "capabilities demo") || strings.Contains(p, "capability demo") || strings.Contains(p, "frontend tool demo") || strings.Contains(p, "showcase")
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
