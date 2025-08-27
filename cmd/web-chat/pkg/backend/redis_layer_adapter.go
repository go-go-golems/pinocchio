package backend

import (
    "github.com/go-go-golems/glazed/pkg/cmds/layers"
    rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
)

// NewRedisParameterLayer exposes the redis parameter layer from the shared package
func NewRedisParameterLayer() (layers.ParameterLayer, error) {
    return rediscfg.NewParameterLayer()
}


