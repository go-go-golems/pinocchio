package webchat

import "sync"

var (
	timelineHandlersBootstrapOnce sync.Once
)

// RegisterDefaultTimelineHandlers installs built-in projection handlers.
// It is safe to call multiple times; registration happens exactly once.
func RegisterDefaultTimelineHandlers() {
	timelineHandlersBootstrapOnce.Do(func() {
		registerBuiltinTimelineHandlers()
		registerThinkingModeTimelineHandlers()
	})
}

func resetTimelineHandlerBootstrapForTests() {
	timelineHandlersBootstrapOnce = sync.Once{}
}
