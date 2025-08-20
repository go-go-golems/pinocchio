package redisstream

import (
	"github.com/ThreeDotsLabs/watermill/message"
	rstream "github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/redis/go-redis/v9"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/helpers"
	"github.com/rs/zerolog/log"
)

// BuildRouter constructs an events.EventRouter backed by Redis Streams when enabled.
// If settings.Enabled is false, it returns a default in-memory router.
func BuildRouter(s Settings, verbose bool) (*events.EventRouter, error) {
	if !s.Enabled {
		return events.NewEventRouter(optVerbose(verbose))
	}

	client := redis.NewClient(&redis.Options{Addr: s.Addr})
	marshaler := rstream.DefaultMarshallerUnmarshaller{}
	logger := helpers.NewWatermill(log.Logger)

	pub, err := rstream.NewPublisher(rstream.PublisherConfig{
		Client:     client,
		Marshaller: marshaler,
	}, logger)
	if err != nil {
		return nil, err
	}

	sub, err := rstream.NewSubscriber(rstream.SubscriberConfig{
		Client:        client,
		Unmarshaller:  marshaler,
		ConsumerGroup: s.Group,
		Consumer:      s.Consumer,
	}, logger)
	if err != nil {
		return nil, err
	}

	return events.NewEventRouter(
		events.WithPublisher(message.Publisher(pub)),
		events.WithSubscriber(message.Subscriber(sub)),
		optVerbose(verbose),
	)
}

// BuildGroupSubscriber returns a Redis Streams subscriber bound to the given consumer group/name.
// Use with events.WithHandlerSubscriber to isolate handlers.
func BuildGroupSubscriber(addr, group, consumer string) (message.Subscriber, error) {
	client := redis.NewClient(&redis.Options{Addr: addr})
	marshaler := rstream.DefaultMarshallerUnmarshaller{}
	logger := helpers.NewWatermill(log.Logger)
	return rstream.NewSubscriber(rstream.SubscriberConfig{
		Client:        client,
		Unmarshaller:  marshaler,
		ConsumerGroup: group,
		Consumer:      consumer,
	}, logger)
}

func optVerbose(v bool) events.EventRouterOption {
	if v {
		return events.WithVerbose(true)
	}
	return func(r *events.EventRouter) {}
}


