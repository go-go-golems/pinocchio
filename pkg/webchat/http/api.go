package webhttp

import (
	nethttp "net/http"

	root "github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// RequestResolver resolves chat/ws request metadata.
type RequestResolver = root.ConversationRequestResolver

// ChatService is the minimal chat submission contract for HTTP helpers.
type ChatService = root.ChatHTTPService

// StreamService is the minimal websocket attach contract for HTTP helpers.
type StreamService = root.StreamHTTPService

// TimelineService is the minimal timeline snapshot contract for HTTP helpers.
type TimelineService = root.TimelineHTTPService

func NewChatHandler(svc ChatService, resolver RequestResolver) nethttp.HandlerFunc {
	return root.NewChatHTTPHandler(svc, resolver)
}

func NewWSHandler(svc StreamService, resolver RequestResolver, upgrader websocket.Upgrader) nethttp.HandlerFunc {
	return root.NewWSHTTPHandler(svc, resolver, upgrader)
}

func NewTimelineHandler(svc TimelineService, logger zerolog.Logger) nethttp.HandlerFunc {
	return root.NewTimelineHTTPHandler(svc, logger)
}
