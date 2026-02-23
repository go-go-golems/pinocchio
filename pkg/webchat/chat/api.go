package chat

import root "github.com/go-go-golems/pinocchio/pkg/webchat"

// Service is the chat-focused API surface.
type Service = root.ChatService

// Config configures chat service construction.
type Config = root.ChatServiceConfig

// ConversationHandle describes ensured conversation metadata.
type ConversationHandle = root.ConversationHandle

// ConversationRuntimeRequest carries conv/runtime resolution inputs.
type ConversationRuntimeRequest = root.ConversationRuntimeRequest

// SubmitPromptInput defines queue/idempotency chat submission input.
type SubmitPromptInput = root.SubmitPromptInput

// SubmitPromptResult is the response contract for chat submission.
type SubmitPromptResult = root.SubmitPromptResult

var (
	NewService                 = root.NewChatService
	NewServiceFromConversation = root.NewChatServiceFromConversation
)
