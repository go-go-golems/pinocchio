package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/types/known/structpb"
)

type frontendToolManifestRequest struct {
	ClientInstanceID string                      `json:"clientInstanceId"`
	ConnectionID     string                      `json:"connectionId"`
	Revision         uint64                      `json:"revision"`
	Tools            []frontendToolManifestEntry `json:"tools"`
}

type frontendToolManifestEntry struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Mode        string         `json:"mode"`
	InputSchema map[string]any `json:"inputSchema"`
	Available   bool           `json:"available"`
}

type frontendToolExecutor struct {
	ClientInstanceID string `json:"clientInstanceId"`
	ConnectionID     string `json:"connectionId"`
	AssignmentID     string `json:"assignmentId"`
}

type frontendToolManifestResponse struct {
	Accepted bool                 `json:"accepted"`
	Revision uint64               `json:"revision"`
	Executor frontendToolExecutor `json:"executor"`
}

type frontendToolResultRequest struct {
	ToolCallID string               `json:"toolCallId"`
	ToolName   string               `json:"toolName"`
	Status     string               `json:"status"`
	Result     any                  `json:"result"`
	Error      string               `json:"error"`
	Executor   frontendToolExecutor `json:"executor"`
}

type frontendToolResultResponse struct {
	Accepted bool   `json:"accepted"`
	Status   string `json:"status"`
}

func (s *Server) handleFrontendToolManifest(w http.ResponseWriter, r *http.Request, c *call) {
	sid := c.sid
	var in frontendToolManifestRequest
	if _, ok := s.decodeBody(w, r, &in); !ok {
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
			writeError(w, http.StatusBadRequest, CodeInvalidRequest, fmt.Errorf("bad input schema for %s: %w", name, err))
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
	if s.frontendTools == nil || s.hub == nil {
		writeError(w, http.StatusNotFound, CodeFrontendToolsDisabled, errors.New("frontend tools are not configured"))
		return
	}
	updated, err := s.frontendTools.AcceptManifest(r.Context(), sid, s.hub, &toolv1.FrontendToolManifestCommand{
		Tools:            descriptors,
		Revision:         in.Revision,
		ClientInstanceId: strings.TrimSpace(in.ClientInstanceID),
		ConnectionId:     strings.TrimSpace(in.ConnectionID),
	})
	if err != nil {
		writeError(w, frontendToolManifestErrorStatus(err), frontendToolErrorCode(err), err)
		return
	}
	writeJSON(w, http.StatusOK, frontendToolManifestResponse{
		Accepted: true,
		Revision: updated.GetRevision(),
		Executor: frontendToolExecutorFromProto(updated.GetExecutor()),
	})
}

func frontendToolManifestErrorStatus(err error) int {
	code, ok := frontendtools.ManifestErrorCodeOf(err)
	if !ok {
		return http.StatusInternalServerError
	}
	switch code {
	case frontendtools.ManifestErrorIdentityMissing, frontendtools.ManifestErrorIdentityTooLong:
		return http.StatusBadRequest
	case frontendtools.ManifestErrorRevisionRegression, frontendtools.ManifestErrorRevisionConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func (executor frontendToolExecutor) toProto() *toolv1.FrontendToolExecutor {
	return &toolv1.FrontendToolExecutor{
		ClientInstanceId: strings.TrimSpace(executor.ClientInstanceID),
		ConnectionId:     strings.TrimSpace(executor.ConnectionID),
		AssignmentId:     strings.TrimSpace(executor.AssignmentID),
	}
}

func frontendToolExecutorFromProto(executor *toolv1.FrontendToolExecutor) frontendToolExecutor {
	return frontendToolExecutor{
		ClientInstanceID: executor.GetClientInstanceId(),
		ConnectionID:     executor.GetConnectionId(),
		AssignmentID:     executor.GetAssignmentId(),
	}
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

func (s *Server) handleFrontendToolResult(w http.ResponseWriter, r *http.Request, c *call) {
	sid := c.sid
	var in frontendToolResultRequest
	if _, ok := s.decodeBody(w, r, &in); !ok {
		return
	}
	in.ToolCallID = strings.TrimSpace(in.ToolCallID)
	in.ToolName = strings.TrimSpace(in.ToolName)
	in.Status = strings.TrimSpace(in.Status)
	if in.ToolCallID == "" {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, errors.New("missing toolCallId"))
		return
	}
	if in.ToolName == "" {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, errors.New("missing toolName"))
		return
	}
	if in.Status == "" {
		in.Status = "success"
	}
	if err := s.publishFrontendToolResult(r.Context(), sid, in); err != nil {
		writeError(w, frontendToolResultErrorStatus(err), frontendToolErrorCode(err), err)
		return
	}
	writeJSON(w, http.StatusOK, frontendToolResultResponse{Accepted: true, Status: in.Status})
}

func frontendToolResultErrorStatus(err error) int {
	code, ok := frontendtools.InvocationErrorCodeOf(err)
	if !ok {
		return http.StatusInternalServerError
	}
	switch code {
	case frontendtools.InvocationErrorInvalidStatus,
		frontendtools.InvocationErrorExecutorMissing:
		return http.StatusBadRequest
	case frontendtools.InvocationErrorUnknownResult:
		return http.StatusNotFound
	case frontendtools.InvocationErrorLateResult:
		return http.StatusGone
	case frontendtools.InvocationErrorDuplicatePending,
		frontendtools.InvocationErrorSessionMismatch,
		frontendtools.InvocationErrorToolMismatch,
		frontendtools.InvocationErrorTerminalConflict,
		frontendtools.InvocationErrorKeyReuse,
		frontendtools.InvocationErrorExecutorMismatch,
		frontendtools.InvocationErrorExecutorUnavailable:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func (s *Server) publishFrontendToolResult(ctx context.Context, sid sessionstream.SessionId, result frontendToolResultRequest) error {
	if s == nil || s.service == nil {
		return fmt.Errorf("server is not initialized")
	}
	resultPayload, err := structpb.NewValue(result.Result)
	if err != nil {
		return fmt.Errorf("encode frontend tool result: %w", err)
	}
	return s.service.SubmitCommand(ctx, sid, frontendtools.CommandResult, &toolv1.FrontendToolResultCommand{
		ToolCallId: result.ToolCallID,
		ToolName:   result.ToolName,
		Result:     resultPayload,
		Status:     result.Status,
		Error:      result.Error,
		Executor:   result.Executor.toProto(),
	})
}

// frontendToolErrorCode is the frontendtools error code of err, if any.
func frontendToolErrorCode(err error) string {
	if code, ok := frontendtools.InvocationErrorCodeOf(err); ok {
		return string(code)
	}
	if code, ok := frontendtools.ManifestErrorCodeOf(err); ok {
		return string(code)
	}
	return CodeInternal
}
