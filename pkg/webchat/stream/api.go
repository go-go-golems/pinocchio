package stream

import (
	"context"
	"errors"

	"github.com/go-go-golems/glazed/pkg/cmds/values"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
	root "github.com/go-go-golems/pinocchio/pkg/webchat"
)

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
	NewBackend = root.NewStreamBackend
	NewHub     = root.NewStreamHub
)

// NewBackendFromValues decodes redis settings from parsed values and builds a stream backend.
// Deprecated: use NewBackend or root.NewStreamBackend with already-decoded redis settings.
func NewBackendFromValues(ctx context.Context, parsed *values.Values) (Backend, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	if parsed == nil {
		return nil, errors.New("parsed values are nil")
	}
	settings := rediscfg.Settings{}
	_ = parsed.DecodeSectionInto("redis", &settings)
	return root.NewStreamBackend(ctx, settings)
}
