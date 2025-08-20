package main

import (
    "github.com/go-go-golems/glazed/pkg/cmds/layers"
    "github.com/go-go-golems/glazed/pkg/cmds/parameters"
)

// RedisSettings holds redis connection & stream config
type RedisSettings struct {
    Addr     string `glazed.parameter:"redis-addr" glazed.default:"localhost:6379" glazed.help:"Redis address host:port"`
    Stream   string `glazed.parameter:"redis-stream" glazed.default:"chat" glazed.help:"Redis stream name"`
    Group    string `glazed.parameter:"redis-group" glazed.default:"chat-ui" glazed.help:"Redis consumer group"`
    Consumer string `glazed.parameter:"redis-consumer" glazed.default:"ui-1" glazed.help:"Redis consumer name"`
}

// BuildRedisLayer returns a LayerDefinition for the command description
func BuildRedisLayer() (layers.ParameterLayer, error) {
    return layers.NewParameterLayer(
        "redis",
        "Redis configuration for Watermill Redis Streams",
        layers.WithParameterDefinitions(
            parameters.NewParameterDefinition("redis-addr", parameters.ParameterTypeString, parameters.WithDefault("localhost:6379")),
            parameters.NewParameterDefinition("redis-stream", parameters.ParameterTypeString, parameters.WithDefault("chat")),
            parameters.NewParameterDefinition("redis-group", parameters.ParameterTypeString, parameters.WithDefault("chat-ui")),
            parameters.NewParameterDefinition("redis-consumer", parameters.ParameterTypeString, parameters.WithDefault("ui-1")),
        ),
    )
}


