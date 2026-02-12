package redisstream

import (
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
)

// Settings holds Redis Streams transport configuration for Watermill.
type Settings struct {
	Enabled  bool   `glazed:"redis-enabled" glazed.default:"false" glazed.help:"Enable Redis Streams transport for events"`
	Addr     string `glazed:"redis-addr" glazed.default:"localhost:6379" glazed.help:"Redis address host:port"`
	Group    string `glazed:"redis-group" glazed.default:"chat-ui" glazed.help:"Redis consumer group"`
	Consumer string `glazed:"redis-consumer" glazed.default:"ui-1" glazed.help:"Redis consumer name"`
}

// NewParameterLayer returns a section definition for Redis Streams settings.
func NewParameterLayer() (schema.Section, error) {
	return schema.NewSection(
		"redis",
		"Redis configuration for Watermill Redis Streams",
		schema.WithFields(
			fields.New("redis-enabled", fields.TypeBool, fields.WithDefault(false)),
			fields.New("redis-addr", fields.TypeString, fields.WithDefault("localhost:6379")),
			fields.New("redis-group", fields.TypeString, fields.WithDefault("chat-ui")),
			fields.New("redis-consumer", fields.TypeString, fields.WithDefault("ui-1")),
		),
	)
}
