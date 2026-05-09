package chatapp

import (
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func runtimeWarningMessageID(messageID string) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return "chat-warning"
	}
	return messageID + ":warning"
}

func isMaxIterationsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "max iterations")
}

func maxIterationsWarningText(err error) string {
	message := "tool loop reached the maximum iteration limit"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = strings.TrimSpace(err.Error())
	}
	return "Warning: inference stopped because " + message + ". The answer may be incomplete; try narrowing the request or increasing the max-iterations setting."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func protoMessageAsMap(msg proto.Message) map[string]any {
	if msg == nil {
		return map[string]any{}
	}
	body, err := protojson.MarshalOptions{EmitUnpopulated: false, UseProtoNames: false}.Marshal(msg)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	out := map[string]any{}
	if err := json.Unmarshal(body, &out); err != nil {
		return map[string]any{"error": err.Error()}
	}
	return out
}
