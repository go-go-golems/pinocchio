package webchat

import (
	"context"
	"time"
)

func (cm *ConvManager) SetEvictionConfig(idle, interval time.Duration) {
	if cm == nil {
		return
	}
	cm.mu.Lock()
	cm.evictIdle = idle
	cm.evictInterval = interval
	cm.mu.Unlock()
}

func (cm *ConvManager) StartEvictionLoop(ctx context.Context) {
	if cm == nil {
		return
	}
	if ctx == nil {
		panic("webchat: StartEvictionLoop requires non-nil ctx")
	}
	cm.mu.Lock()
	if cm.evictRunning {
		cm.mu.Unlock()
		return
	}
	idle := cm.evictIdle
	interval := cm.evictInterval
	if idle <= 0 || interval <= 0 {
		cm.mu.Unlock()
		return
	}
	cm.evictRunning = true
	cm.mu.Unlock()

	go cm.runEvictionLoop(ctx, interval)
}

func (cm *ConvManager) runEvictionLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			cm.mu.Lock()
			cm.evictRunning = false
			cm.mu.Unlock()
			return
		case now := <-ticker.C:
			cm.evictIdleOnce(now)
		}
	}
}

func (cm *ConvManager) evictIdleOnce(now time.Time) int {
	if cm == nil {
		return 0
	}
	if now.IsZero() {
		now = time.Now()
	}

	cm.mu.Lock()
	idle := cm.evictIdle
	if idle <= 0 {
		cm.mu.Unlock()
		return 0
	}
	convs := make([]*Conversation, 0, len(cm.conns))
	for _, conv := range cm.conns {
		convs = append(convs, conv)
	}
	cm.mu.Unlock()

	evicted := 0
	for _, conv := range convs {
		if conv == nil {
			continue
		}
		if !cm.shouldEvictConversation(now, idle, conv) {
			continue
		}
		cm.mu.Lock()
		current, ok := cm.conns[conv.ID]
		if !ok || current != conv {
			cm.mu.Unlock()
			continue
		}
		delete(cm.conns, conv.ID)
		cm.mu.Unlock()

		cm.cleanupConversation(conv)
		evicted++
	}

	return evicted
}

func (cm *ConvManager) shouldEvictConversation(now time.Time, idle time.Duration, conv *Conversation) bool {
	if conv.pool != nil && !conv.pool.IsEmpty() {
		return false
	}
	if conv.stream != nil && conv.stream.IsRunning() {
		return false
	}
	conv.mu.Lock()
	busy := conv.isBusyLocked()
	queueLen := len(conv.queue)
	last := conv.lastActivity
	conv.mu.Unlock()
	if busy || queueLen > 0 {
		return false
	}
	if last.IsZero() {
		return false
	}
	return now.Sub(last) >= idle
}

func (cm *ConvManager) cleanupConversation(conv *Conversation) {
	if conv == nil {
		return
	}
	if conv.pool != nil {
		conv.pool.CloseAll()
	}
	if conv.stream != nil {
		if conv.subClose {
			conv.stream.Close()
		} else {
			conv.stream.Stop()
		}
	}
}
