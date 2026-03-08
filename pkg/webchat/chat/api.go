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

// StartRequest contains the transport-safe envelope passed to a Runner.
type StartRequest = root.StartRequest

// StartResult captures the immediate response from a Runner start.
type StartResult = root.StartResult

// Runner starts a process against an ensured conversation transport.
type Runner = root.Runner

// StartPromptWithRunnerInput defines queue/idempotency aware prompt submission over a Runner.
type StartPromptWithRunnerInput = root.StartPromptWithRunnerInput

// SubmitPromptInput defines queue/idempotency chat submission input.
type SubmitPromptInput = root.SubmitPromptInput

// SubmitPromptResult is the response contract for chat submission.
type SubmitPromptResult = root.SubmitPromptResult

var (
	NewService                 = root.NewChatService
	NewServiceFromConversation = root.NewChatServiceFromConversation
)
