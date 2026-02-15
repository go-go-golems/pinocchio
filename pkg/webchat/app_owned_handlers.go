package webchat

import (
	"net/http"

	"github.com/gorilla/websocket"
)

func NewChatHandler(svc *ConversationService, resolver ConversationRequestResolver) http.HandlerFunc {
	return NewChatHTTPHandler(svc, resolver)
}

func NewWSHandler(svc *ConversationService, resolver ConversationRequestResolver, upgrader websocket.Upgrader) http.HandlerFunc {
	return NewWSHTTPHandler(svc, resolver, upgrader)
}
