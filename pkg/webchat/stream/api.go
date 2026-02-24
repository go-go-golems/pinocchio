package stream

import root "github.com/go-go-golems/pinocchio/pkg/webchat"

// Hub owns per-conversation streaming lifecycle.
type Hub = root.StreamHub

// HubConfig configures stream hub dependencies.
type HubConfig = root.StreamHubConfig

// Backend abstracts in-memory/redis stream transport.
type Backend = root.StreamBackend

// Cursor captures stream sequence metadata.
type Cursor = root.StreamCursor

// WebSocketAttachOptions controls hello/ping/pong behavior.
type WebSocketAttachOptions = root.WebSocketAttachOptions

// ConversationHandle describes ensured conversation metadata.
type ConversationHandle = root.ConversationHandle

// ConversationRuntimeRequest carries conv/runtime resolution inputs.
type ConversationRuntimeRequest = root.ConversationRuntimeRequest

var (
	NewHub               = root.NewStreamHub
	NewBackendFromValues = root.NewStreamBackendFromValues
)
