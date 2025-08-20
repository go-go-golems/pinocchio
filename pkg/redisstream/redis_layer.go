package redisstream

import (
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
)

// Settings holds Redis Streams transport configuration for Watermill.
type Settings struct {
	Enabled  bool   `glazed.parameter:"redis-enabled" glazed.default:"false" glazed.help:"Enable Redis Streams transport for events"`
	Addr     string `glazed.parameter:"redis-addr" glazed.default:"localhost:6379" glazed.help:"Redis address host:port"`
	Group    string `glazed.parameter:"redis-group" glazed.default:"chat-ui" glazed.help:"Redis consumer group"`
	Consumer string `glazed.parameter:"redis-consumer" glazed.default:"ui-1" glazed.help:"Redis consumer name"`
}

// NewParameterLayer returns a LayerDefinition for Redis Streams settings.
func NewParameterLayer() (layers.ParameterLayer, error) {
	return layers.NewParameterLayer(
		"redis",
		"Redis configuration for Watermill Redis Streams",
		layers.WithParameterDefinitions(
			parameters.NewParameterDefinition("redis-enabled", parameters.ParameterTypeBool, parameters.WithDefault(false)),
			parameters.NewParameterDefinition("redis-addr", parameters.ParameterTypeString, parameters.WithDefault("localhost:6379")),
			parameters.NewParameterDefinition("redis-group", parameters.ParameterTypeString, parameters.WithDefault("chat-ui")),
			parameters.NewParameterDefinition("redis-consumer", parameters.ParameterTypeString, parameters.WithDefault("ui-1")),
		),
	)
}


