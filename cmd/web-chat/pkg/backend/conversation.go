package backend

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/ThreeDotsLabs/watermill/message"
    "github.com/gorilla/websocket"
    "github.com/google/uuid"
    "github.com/rs/zerolog/log"

    "github.com/go-go-golems/geppetto/pkg/events"
    "github.com/go-go-golems/geppetto/pkg/inference/engine"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/turns"
    rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
)

// Conversation holds per-conversation runtime state
type Conversation struct {
    ID        string
    RunID     string
    Turn      *turns.Turn
    Eng       engine.Engine
    Sink      *middleware.WatermillSink
    running   bool
    cancel    context.CancelFunc
    mu        sync.Mutex
    conns     map[*websocket.Conn]bool
    connsMu   sync.RWMutex
    sub       message.Subscriber
    stopRead  context.CancelFunc
    reading   bool
    idleTimer *time.Timer
}

// ConvManager keeps track of existing conversations
type ConvManager struct {
    mu    sync.Mutex
    conns map[string]*Conversation
}

// getOrCreateConv returns an existing conversation or creates a new one with engine and sink
func (s *Server) getOrCreateConv(convID string) (*Conversation, error) {
    s.cm.mu.Lock()
    defer s.cm.mu.Unlock()
    if conv, ok := s.cm.conns[convID]; ok {
        return conv, nil
    }
    runID := uuid.NewString()
    conv := &Conversation{ID: convID, RunID: runID, conns: make(map[*websocket.Conn]bool)}
    // Create dedicated subscriber per conversation
    if s.redis.Enabled {
        _ = rediscfg.EnsureGroupAtTail(context.Background(), s.redis.Addr, s.topicForConv(convID), "ui")
        s_, err := rediscfg.BuildGroupSubscriber(s.redis.Addr, "ui", "ws-forwarder:"+convID)
        if err != nil {
            return nil, err
        }
        conv.sub = s_
    } else {
        conv.sub = s.router.Subscriber
    }
    // Build per-conversation engine and sink
    {
        eng, err := s.buildEngine()
        if err != nil {
            return nil, err
        }
        conv.Eng = eng
        conv.Sink = middleware.NewWatermillSink(s.router.Publisher, s.topicForConv(conv.ID))
    }
    // Initialize long-lived turn
    conv.Turn = &turns.Turn{RunID: conv.RunID, Data: map[string]any{}}
    if err := s.startReader(conv); err != nil {
        return nil, err
    }
    s.cm.conns[convID] = conv
    return conv, nil
}

// startReader subscribes to the per-conversation topic and forwards events to websocket clients
func (s *Server) startReader(conv *Conversation) error {
    if conv.reading {
        return nil
    }
    log.Info().Str("conv_id", conv.ID).Str("topic", s.topicForConv(conv.ID)).Msg("starting conversation reader")
    readCtx, readCancel := context.WithCancel(context.Background())
    conv.stopRead = readCancel
    ch, err := conv.sub.Subscribe(readCtx, s.topicForConv(conv.ID))
    if err != nil {
        readCancel()
        conv.stopRead = nil
        return err
    }
    conv.reading = true
    go func() {
        for msg := range ch {
            e, err := events.NewEventFromJson(msg.Payload)
            if err != nil {
                log.Warn().Err(err).Str("component", "ws_reader").Msg("failed to decode event json")
                msg.Ack()
                continue
            }
            runID := e.Metadata().RunID
            if runID != "" && runID != conv.RunID {
                log.Debug().Str("component", "ws_reader").Str("event_type", fmt.Sprintf("%T", e)).Str("event_id", e.Metadata().ID.String()).Str("run_id", runID).Str("conv_run_id", conv.RunID).Msg("skipping event due to run_id mismatch")
                msg.Ack()
                continue
            }
            log.Debug().Str("component", "ws_reader").Str("event_type", fmt.Sprintf("%T", e)).Str("event_id", e.Metadata().ID.String()).Str("run_id", runID).Msg("forwarding event to timeline")
            // Inline debug log handler
            switch ev := e.(type) {
            case *events.EventToolCall:
                log.Info().Str("tool", ev.ToolCall.Name).Str("id", ev.ToolCall.ID).Str("input", ev.ToolCall.Input).Msg("ToolCall")
            case *events.EventToolCallExecute:
                log.Info().Str("tool", ev.ToolCall.Name).Str("id", ev.ToolCall.ID).Str("input", ev.ToolCall.Input).Msg("ToolExecute")
            case *events.EventToolResult:
                log.Info().Str("tool_result_id", ev.ToolResult.ID).Interface("result", ev.ToolResult.Result).Msg("ToolResult")
            case *events.EventToolCallExecutionResult:
                log.Info().Str("tool_result_id", ev.ToolResult.ID).Interface("result", ev.ToolResult.Result).Msg("ToolExecResult")
            case *events.EventLog:
                lvl := ev.Level
                if lvl == "" {
                    lvl = "info"
                }
                log.WithLevel(parseZerologLevel(lvl)).Str("message", ev.Message).Fields(ev.Fields).Msg("LogEvent")
            case *events.EventInfo:
                log.Info().Str("message", ev.Message).Fields(ev.Data).Msg("InfoEvent")
            }
            s.convertAndBroadcast(conv, e)
            msg.Ack()
        }
        conv.mu.Lock()
        conv.reading = false
        conv.stopRead = nil
        conv.mu.Unlock()
        log.Info().Str("conv_id", conv.ID).Msg("conversation reader stopped")
    }()
    return nil
}

// convertAndBroadcast converts an event into frontend frames and sends them to all sockets
func (s *Server) convertAndBroadcast(conv *Conversation, e events.Event) {
    sendBytes := func(b []byte) {
        conv.connsMu.RLock()
        for c := range conv.conns {
            _ = c.WriteMessage(websocket.TextMessage, b)
        }
        conv.connsMu.RUnlock()
    }
    if bs := SemanticEventsFromEvent(e); bs != nil {
        for _, b := range bs {
            log.Debug().Str("component", "ws_broadcast").Int("bytes", len(b)).Msg("broadcasting semantic event")
            sendBytes(b)
        }
    }
}

// addConn adds a websocket connection to the conversation and ensures the reader is running
func (s *Server) addConn(conv *Conversation, c *websocket.Conn) {
    conv.connsMu.Lock()
    conv.conns[c] = true
    conv.connsMu.Unlock()
    conv.mu.Lock()
    if conv.idleTimer != nil {
        conv.idleTimer.Stop()
        conv.idleTimer = nil
    }
    wasReading := conv.reading
    conv.mu.Unlock()
    if !wasReading && s.redis.Enabled {
        _ = s.startReader(conv)
    }
}

// removeConn removes a websocket connection and schedules reader stop on idle
func (s *Server) removeConn(conv *Conversation, c *websocket.Conn) {
    conv.connsMu.Lock()
    delete(conv.conns, c)
    conv.connsMu.Unlock()
    _ = c.Close()
    if s.settings.IdleTimeoutSeconds > 0 {
        conv.connsMu.RLock()
        empty := len(conv.conns) == 0
        conv.connsMu.RUnlock()
        if empty {
            conv.mu.Lock()
            if conv.idleTimer == nil {
                d := time.Duration(s.settings.IdleTimeoutSeconds) * time.Second
                conv.idleTimer = time.AfterFunc(d, func() {
                    conv.mu.Lock()
                    defer conv.mu.Unlock()
                    conv.connsMu.RLock()
                    isEmpty := len(conv.conns) == 0
                    conv.connsMu.RUnlock()
                    if isEmpty && conv.stopRead != nil {
                        log.Info().Str("conv_id", conv.ID).Msg("idle timeout reached; stopping conversation reader")
                        conv.stopRead()
                        conv.stopRead = nil
                        conv.reading = false
                    }
                })
                log.Debug().Str("conv_id", conv.ID).Int("idle_sec", s.settings.IdleTimeoutSeconds).Msg("scheduled reader stop after idle period")
            }
            conv.mu.Unlock()
        }
    }
}


