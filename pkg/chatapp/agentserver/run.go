package agentserver

import (
	"context"
	"encoding/json"
	"time"

	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/httpapi"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

type runKey struct{}

// keepRun is a chatapp.RunToolsFunc that keeps the run (session, message id,
// publisher) in the tools' context, for Session and tool progress events.
func keepRun(ctx context.Context, scope chatapp.RunScope, tools chatapp.RunTools) (context.Context, chatapp.RunTools, error) {
	return context.WithValue(ctx, runKey{}, scope), tools, nil
}

func runOf(ctx context.Context) (chatapp.RunScope, bool) {
	scope, ok := ctx.Value(runKey{}).(chatapp.RunScope)
	return scope, ok && scope.SessionID != ""
}

// Session is the session of the run an in-process tool is called in (its
// labels and data), read from the session store.
func (s *Server) Session(ctx context.Context) (sessions.Info, bool) {
	scope, ok := runOf(ctx)
	if !ok || s.api == nil {
		return sessions.Info{}, false
	}
	info, err := s.api.Sessions().Get(ctx, string(scope.SessionID))
	if err != nil {
		return sessions.Info{}, false
	}
	return info, true
}

// publishToolState is toolexec's OnState: a call that starts or stops
// waiting for a person becomes a ChatToolCallProgress event of its run.
func publishToolState(ctx context.Context, call geptools.ToolCall, state string, limit time.Duration) {
	scope, ok := runOf(ctx)
	if !ok || scope.Publisher == nil {
		return
	}
	_ = chatapp.Stamped(scope.Publisher).Publish(context.WithoutCancel(ctx), sessionstream.Event{
		Name:      chatapp.EventChatToolCallProgress,
		SessionId: scope.SessionID,
		Payload: &chatappv1.ChatToolCallProgress{
			MessageId:  scope.MessageID,
			ToolCallId: call.ID,
			State:      state,
			LimitMs:    limit.Milliseconds(),
		},
	})
}

// setServerInstructions writes the embedding program's instructions into a
// new session's data (serverInstructions), replacing whatever the page sent
// under that key.
func (s *Server) setServerInstructions(ctx context.Context, info *sessions.Info) error {
	text := ""
	if s.sessionInstructions != nil {
		var err error
		if text, err = s.sessionInstructions(ctx, *info); err != nil {
			return err
		}
	}
	fields := map[string]json.RawMessage{}
	if len(info.Data) > 0 && string(info.Data) != "null" {
		if err := json.Unmarshal(info.Data, &fields); err != nil {
			return httpapi.Coded(httpapi.CodeInvalidRequest, "session data must be a JSON object: "+err.Error())
		}
	}
	if _, had := fields["serverInstructions"]; !had && text == "" {
		return nil
	}
	delete(fields, "serverInstructions")
	if text != "" {
		raw, err := json.Marshal(text)
		if err != nil {
			return err
		}
		fields["serverInstructions"] = raw
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	info.Data = raw
	return nil
}
