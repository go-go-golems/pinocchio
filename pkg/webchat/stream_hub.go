package webchat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	sempb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/base"
)

type StreamHubConfig struct {
	BaseCtx      context.Context
	ConvManager  *ConvManager
	SendNotifier WSPublisher
}

// StreamHub owns websocket/stream attachment for per-conversation state.
type StreamHub struct {
	baseCtx context.Context
	cm      *ConvManager
}

func NewStreamHub(cfg StreamHubConfig) (*StreamHub, error) {
	if cfg.BaseCtx == nil {
		return nil, errors.New("stream hub base context is nil")
	}
	if cfg.ConvManager == nil {
		return nil, errors.New("stream hub conv manager is nil")
	}
	return &StreamHub{
		baseCtx: cfg.BaseCtx,
		cm:      cfg.ConvManager,
	}, nil
}

func (h *StreamHub) ResolveAndEnsureConversation(ctx context.Context, req AppConversationRequest) (*ConversationHandle, error) {
	if h == nil || h.cm == nil {
		return nil, errors.New("stream hub is not initialized")
	}
	convID := strings.TrimSpace(req.ConvID)
	if convID == "" {
		convID = uuid.NewString()
	}
	runtimeKey := strings.TrimSpace(req.RuntimeKey)
	if runtimeKey == "" {
		runtimeKey = "default"
	}
	conv, err := h.cm.GetOrCreate(convID, runtimeKey, req.Overrides, req.ResolvedRuntime)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, errors.New("conversation not available")
	}
	return &ConversationHandle{
		ConvID:             conv.ID,
		SessionID:          conv.SessionID,
		RuntimeKey:         conv.RuntimeKey,
		RuntimeFingerprint: conv.RuntimeFingerprint,
		SeedSystemPrompt:   conv.SeedSystemPrompt,
		AllowedTools:       append([]string(nil), conv.AllowedTools...),
	}, nil
}

func (h *StreamHub) AttachWebSocket(ctx context.Context, convID string, conn *websocket.Conn, opts WebSocketAttachOptions) error {
	if h == nil || h.cm == nil {
		return errors.New("stream hub is not initialized")
	}
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return errors.New("missing convID")
	}
	if conn == nil {
		return errors.New("websocket connection is nil")
	}

	conv, ok := h.cm.GetConversation(convID)
	if !ok || conv == nil {
		var err error
		conv, err = h.cm.GetOrCreate(convID, "default", nil, nil)
		if err != nil {
			return err
		}
	}

	h.cm.AddConn(conv, conn)
	wsLog := log.With().
		Str("component", "webchat").
		Str("remote", conn.RemoteAddr().String()).
		Str("conv_id", convID).
		Str("runtime_key", conv.RuntimeKey).
		Logger()

	if opts.SendHello && conv != nil && conv.pool != nil {
		ts := time.Now().UnixMilli()
		data, _ := protoToRaw(&sempb.WsHelloV1{ConvId: convID, RuntimeKey: conv.RuntimeKey, ServerTime: ts})
		hello := map[string]any{
			"sem": true,
			"event": map[string]any{
				"type": "ws.hello",
				"id":   fmt.Sprintf("ws.hello:%s:%d", convID, ts),
				"data": data,
			},
		}
		if b, err := json.Marshal(hello); err == nil {
			wsLog.Debug().Msg("ws sending hello")
			conv.pool.SendToOne(conn, b)
		}
	}

	go func() {
		defer h.cm.RemoveConn(conv, conn)
		defer wsLog.Info().Msg("ws disconnected")
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				wsLog.Debug().Err(err).Msg("ws read loop end")
				return
			}

			if opts.HandlePingPong && msgType == websocket.TextMessage && len(data) > 0 && conv != nil && conv.pool != nil {
				text := strings.TrimSpace(strings.ToLower(string(data)))
				isPing := text == "ping"
				if !isPing {
					var v map[string]any
					if err := json.Unmarshal(data, &v); err == nil && v != nil {
						if t, ok := v["type"].(string); ok && strings.EqualFold(t, "ws.ping") {
							isPing = true
						} else if sem, ok := v["sem"].(bool); ok && sem {
							if ev, ok := v["event"].(map[string]any); ok {
								if t2, ok := ev["type"].(string); ok && strings.EqualFold(t2, "ws.ping") {
									isPing = true
								}
							}
						}
					}
				}
				if isPing {
					ts := time.Now().UnixMilli()
					data, _ := protoToRaw(&sempb.WsPongV1{ConvId: convID, ServerTime: ts})
					pong := map[string]any{
						"sem": true,
						"event": map[string]any{
							"type": "ws.pong",
							"id":   fmt.Sprintf("ws.pong:%s:%d", convID, ts),
							"data": data,
						},
					}
					if b, err := json.Marshal(pong); err == nil {
						wsLog.Debug().Msg("ws sending pong")
						conv.pool.SendToOne(conn, b)
					}
				}
			}
		}
	}()
	return nil
}
