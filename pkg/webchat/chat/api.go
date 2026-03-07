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

// PrepareRunnerStartInput defines the ensured-conversation inputs used before a Runner starts.
type PrepareRunnerStartInput = root.PrepareRunnerStartInput

// StartRequest contains the per-conversation surfaces supplied to a Runner.
type StartRequest = root.StartRequest

// SubmitPromptInput defines queue/idempotency chat submission input.
//
//nolint:staticcheck // compatibility alias for the deprecated legacy chat startup path
type SubmitPromptInput = root.SubmitPromptInput

// SubmitPromptResult is the response contract for chat submission.
//
//nolint:staticcheck // compatibility alias for the deprecated legacy chat startup path
type SubmitPromptResult = root.SubmitPromptResult

var (
	NewService                 = root.NewChatService
	NewServiceFromConversation = root.NewChatServiceFromConversation
)
