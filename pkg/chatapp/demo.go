package chatapp

import (
	"context"
	"strings"
	"time"

	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	widgetv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/widgets/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/types/known/structpb"
)

func (e *Engine) runDemoInference(ctx context.Context, sid sessionstream.SessionId, messageID, prompt string, pub sessionstream.EventPublisher) {
	if isCapabilitiesShowcasePrompt(prompt) {
		e.runCapabilitiesShowcase(ctx, sid, messageID, prompt, pub)
		return
	}
	if err := e.publish(ctx, sid, pub, EventChatRunStarted, &chatappv1.ChatRunStarted{MessageId: messageID, Prompt: prompt, Correlation: runCorrelationInfo(sid, messageID)}); err != nil {
		return
	}

	answer := renderAnswer(prompt)
	chunks := chunkText(answer, 10)
	accumulated := ""
	textMessageID := textSegmentMessageID(messageID, 1)
	corr := &chatappv1.CorrelationInfo{SessionId: string(sid), RunId: messageID, SegmentId: textMessageID}
	if err := e.publish(publishContext(ctx), sid, pub, EventChatTextSegmentStarted, &chatappv1.ChatTextSegmentStarted{MessageId: textMessageID, Role: "assistant", Prompt: prompt, Status: "streaming", Streaming: true, Correlation: corr}); err != nil {
		return
	}
	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			_ = e.publish(publishContext(ctx), sid, pub, EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{MessageId: textMessageID, Role: "assistant", Prompt: prompt, Text: accumulated, Content: accumulated, Status: "stopped", Streaming: false, Final: true, FinishReason: "stopped", Correlation: corr})
			_ = e.publish(publishContext(ctx), sid, pub, EventChatRunStopped, &chatappv1.ChatRunStopped{MessageId: messageID, Status: "stopped", Correlation: runCorrelationInfo(sid, messageID)})
			return
		case <-time.After(e.chunkDelay):
		}
		accumulated += chunk
		if err := e.publish(publishContext(ctx), sid, pub, EventChatTextPatch, &chatappv1.ChatTextPatch{MessageId: textMessageID, Role: "assistant", Prompt: prompt, StreamId: textMessageID, Sequence: Uint64FromInt(i + 1), Offset: PatchOffset(accumulated, chunk), Text: chunk, Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND, Status: "streaming", Correlation: corr}); err != nil {
			return
		}
	}
	if err := e.publish(publishContext(ctx), sid, pub, EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{MessageId: textMessageID, Role: "assistant", Prompt: prompt, Text: accumulated, Content: accumulated, Status: "finished", Streaming: false, Final: true, Correlation: corr}); err != nil {
		return
	}
	_ = e.publish(publishContext(ctx), sid, pub, EventChatRunFinished, &chatappv1.ChatRunFinished{MessageId: messageID, Status: "finished", Correlation: runCorrelationInfo(sid, messageID)})
}

func isCapabilitiesShowcasePrompt(prompt string) bool {
	p := strings.ToLower(strings.TrimSpace(prompt))
	return strings.Contains(p, "capabilities demo") || strings.Contains(p, "capability demo") || strings.Contains(p, "frontend tool demo") || strings.Contains(p, "showcase")
}

func (e *Engine) runCapabilitiesShowcase(ctx context.Context, sid sessionstream.SessionId, messageID, prompt string, pub sessionstream.EventPublisher) {
	corr := runCorrelationInfo(sid, messageID)
	if err := e.publish(ctx, sid, pub, EventChatRunStarted, &chatappv1.ChatRunStarted{MessageId: messageID, Prompt: prompt, Correlation: corr}); err != nil {
		return
	}

	widgetID := messageID + ":widget:capabilities"
	toolCallID := messageID + ":frontend-tool:confirm"
	textID := textSegmentMessageID(messageID, 1)
	textCorr := &chatappv1.CorrelationInfo{SessionId: string(sid), RunId: messageID, SegmentId: textID}
	intro := "I will demonstrate a streamed typed widget and a browser-owned frontend tool call."
	if err := e.publish(publishContext(ctx), sid, pub, EventChatTextSegmentStarted, &chatappv1.ChatTextSegmentStarted{MessageId: textID, Role: "assistant", Prompt: prompt, Status: "streaming", Streaming: true, Correlation: textCorr}); err != nil {
		return
	}
	if err := e.publish(publishContext(ctx), sid, pub, EventChatTextPatch, &chatappv1.ChatTextPatch{MessageId: textID, Role: "assistant", Prompt: prompt, StreamId: textID, Sequence: Uint64FromInt(1), Offset: 0, Text: intro, Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND, Status: "streaming", Correlation: textCorr}); err != nil {
		return
	}
	if err := e.publish(publishContext(ctx), sid, pub, EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{MessageId: textID, Role: "assistant", Prompt: prompt, Text: intro, Content: intro, Status: "finished", Streaming: false, Final: true, Correlation: textCorr}); err != nil {
		return
	}

	if err := e.publish(publishContext(ctx), sid, pub, "ChatWidgetInstanceStarted", &widgetv1.WidgetInstanceStarted{
		InstanceId:      widgetID,
		WidgetName:      "demo.capability_card",
		ParentMessageId: messageID,
		Status:          widgetv1.WidgetStatus_WIDGET_STATUS_STREAMING,
		Props: capabilityShowcaseProps("Capabilities showcase", "starting", "Starting the web-chat package capabilities demo.", []map[string]any{
			{"id": "text", "label": "Stream assistant text", "state": "done"},
			{"id": "widget", "label": "Render a typed custom widget", "state": "running"},
			{"id": "tool", "label": "Ask the browser for confirmation", "state": "pending"},
			{"id": "finish", "label": "Complete the run", "state": "pending"},
		}, "", ""),
	}); err != nil {
		return
	}

	select {
	case <-ctx.Done():
		_ = e.publish(publishContext(ctx), sid, pub, EventChatRunStopped, &chatappv1.ChatRunStopped{MessageId: messageID, Status: "stopped", Correlation: corr})
		return
	case <-time.After(e.chunkDelay * 3):
	}

	_ = e.publish(publishContext(ctx), sid, pub, "ChatWidgetInstancePatched", &widgetv1.WidgetInstancePatched{
		InstanceId: widgetID,
		WidgetName: "demo.capability_card",
		Status:     widgetv1.WidgetStatus_WIDGET_STATUS_STREAMING,
		Patch: capabilityShowcaseProps("Capabilities showcase", "waiting_for_user", "Waiting for the browser-hosted confirmation tool.", []map[string]any{
			{"id": "text", "label": "Stream assistant text", "state": "done"},
			{"id": "widget", "label": "Render a typed custom widget", "state": "done"},
			{"id": "tool", "label": "Ask the browser for confirmation", "state": "running"},
			{"id": "finish", "label": "Complete the run", "state": "pending"},
		}, toolCallID, ""),
	})

	input, _ := structpb.NewStruct(map[string]any{
		"title":        "Approve the capabilities demo?",
		"body":         "This frontend tool call is rendered and answered by the browser, then reported back to the Pinocchio backend.",
		"confirmLabel": "Approve demo",
		"cancelLabel":  "Deny",
	})
	_ = e.publish(publishContext(ctx), sid, pub, "ChatFrontendToolCallRequested", &toolv1.FrontendToolCallRequested{
		MessageId:  messageID,
		ToolCallId: toolCallID,
		ToolName:   "browser.confirm_action",
		Input:      input,
		Mode:       toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_HUMAN,
		Status:     "requested",
	})

	select {
	case <-ctx.Done():
		_ = e.publish(publishContext(ctx), sid, pub, EventChatRunStopped, &chatappv1.ChatRunStopped{MessageId: messageID, Status: "stopped", Correlation: corr})
		return
	case <-time.After(e.chunkDelay * 8):
	}

	_ = e.publish(publishContext(ctx), sid, pub, "ChatWidgetInstancePatched", &widgetv1.WidgetInstancePatched{
		InstanceId: widgetID,
		WidgetName: "demo.capability_card",
		Status:     widgetv1.WidgetStatus_WIDGET_STATUS_READY,
		Patch: capabilityShowcaseProps("Capabilities showcase", "complete", "The widget stream and frontend tool request were delivered. Approve the tool card to publish a browser result event.", []map[string]any{
			{"id": "text", "label": "Stream assistant text", "state": "done"},
			{"id": "widget", "label": "Render a typed custom widget", "state": "done"},
			{"id": "tool", "label": "Ask the browser for confirmation", "state": "done"},
			{"id": "finish", "label": "Complete the run", "state": "done"},
		}, toolCallID, "Ready for browser approval result."),
	})
	_ = e.publish(publishContext(ctx), sid, pub, "ChatWidgetInstanceCompleted", &widgetv1.WidgetInstanceCompleted{InstanceId: widgetID, Status: widgetv1.WidgetStatus_WIDGET_STATUS_READY})
	_ = e.publish(publishContext(ctx), sid, pub, EventChatRunFinished, &chatappv1.ChatRunFinished{MessageId: messageID, Status: "finished", Correlation: corr})
}

func capabilityShowcaseProps(title, status, summary string, steps []map[string]any, toolCallID, result string) *structpb.Struct {
	payload := map[string]any{
		"title":   title,
		"status":  status,
		"summary": summary,
		"steps":   steps,
	}
	if toolCallID != "" {
		payload["toolCallId"] = toolCallID
	}
	if result != "" {
		payload["result"] = result
	}
	props, err := structpb.NewStruct(payload)
	if err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return props
}

func renderAnswer(prompt string) string {
	return "Answer: " + prompt
}

func chunkText(text string, size int) []string {
	if size <= 0 || len(text) <= size {
		return []string{text}
	}
	out := make([]string, 0, (len(text)+size-1)/size)
	for len(text) > 0 {
		if len(text) <= size {
			out = append(out, text)
			break
		}
		out = append(out, text[:size])
		text = text[size:]
	}
	return out
}
