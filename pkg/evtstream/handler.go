package evtstream

import "context"

// CommandHandler services one registered command name.
type CommandHandler func(ctx context.Context, cmd Command, sess *Session, pub EventPublisher) error

// EventPublisher publishes canonical backend events.
type EventPublisher interface {
	Publish(ctx context.Context, ev Event) error
}
