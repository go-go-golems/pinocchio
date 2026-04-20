package chat

import (
	"time"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	appchat "github.com/go-go-golems/pinocchio/pkg/evtstream/apps/chat"
)

const (
	CommandStartInference = appchat.CommandStartInference
	CommandStopInference  = appchat.CommandStopInference

	EventInferenceStarted  = appchat.EventInferenceStarted
	EventTokensDelta       = appchat.EventTokensDelta
	EventInferenceFinished = appchat.EventInferenceFinished
	EventInferenceStopped  = appchat.EventInferenceStopped

	UIMessageStarted  = appchat.UIMessageStarted
	UIMessageAppended = appchat.UIMessageAppended
	UIMessageFinished = appchat.UIMessageFinished
	UIMessageStopped  = appchat.UIMessageStopped

	TimelineEntityChatMessage = appchat.TimelineEntityChatMessage
)

type Hooks = appchat.Hooks

type Option = appchat.Option

type Engine = appchat.Engine

type Service = appchat.Service

func WithChunkDelay(delay time.Duration) Option {
	return appchat.WithChunkDelay(delay)
}

func WithHooks(h Hooks) Option {
	return appchat.WithHooks(h)
}

func NewEngine(opts ...Option) *Engine {
	return appchat.NewEngine(opts...)
}

func RegisterSchemas(reg *evtstream.SchemaRegistry) error {
	return appchat.RegisterSchemas(reg)
}

func Install(hub *evtstream.Hub, engine *Engine) error {
	return appchat.Install(hub, engine)
}

func NewService(hub *evtstream.Hub, engine *Engine) (*Service, error) {
	return appchat.NewService(hub, engine)
}
