package conversation

import (
	"github.com/go-go-golems/geppetto/pkg/conversation"
)

// ConvertMessage converts a conversation.Message to a WebMessage
func ConvertMessage(msg *conversation.Message) (*WebMessage, error) {
	webMsg := &WebMessage{
		ID:         msg.ID.String(),
		ParentID:   msg.ParentID.String(),
		Time:       msg.Time,
		LastUpdate: msg.LastUpdate,
		Metadata:   msg.Metadata,
	}

	switch content := msg.Content.(type) {
	case *conversation.ChatMessageContent:
		images := make([]string, len(content.Images))
		for i, img := range content.Images {
			if img.ImageURL != "" {
				images[i] = img.ImageURL
			}
			// TODO: handle local images by serving them through a static file server
		}
		webMsg.Content = &WebChatMessage{
			Role:   string(content.Role),
			Text:   content.Text,
			Images: images,
		}
		webMsg.Type = "chat"

	case *conversation.ToolUseContent:
		webMsg.Content = &WebToolUseMessage{
			ToolID: content.ToolID,
			Name:   content.Name,
			Input:  content.Input,
		}
		webMsg.Type = "tool-use"

	case *conversation.ToolResultContent:
		webMsg.Content = &WebToolResultMessage{
			ToolID: content.ToolID,
			Result: content.Result,
		}
		webMsg.Type = "tool-result"

	default:
		// For unknown content types, create a chat message with the string representation
		webMsg.Content = &WebChatMessage{
			Role: "system",
			Text: content.String(),
		}
		webMsg.Type = "chat"
	}

	return webMsg, nil
}

// ConvertConversation converts a conversation.Conversation to a WebConversation
func ConvertConversation(conv conversation.Conversation) (*WebConversation, error) {
	webConv := &WebConversation{
		Messages: make([]*WebMessage, 0, len(conv)),
	}

	for _, msg := range conv {
		webMsg, err := ConvertMessage(msg)
		if err != nil {
			return nil, err
		}
		webConv.Messages = append(webConv.Messages, webMsg)
	}

	return webConv, nil
}
