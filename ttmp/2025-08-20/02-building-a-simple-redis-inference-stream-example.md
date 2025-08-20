---
Title: Building a Simple Redis Streaming Inference Example (Pinocchio)
Slug: simple-redis-streaming-inference
Short: Step-by-step guide to mirror the simple streaming example, add a custom Glaze layer for Redis, and stream Geppetto events via Redis Streams.
Topics:
- pinocchio
- geppetto
- events
- watermill
- redis
- glaze
SectionType: HowTo
Date: 2025-08-20
---

## Goal

Create a new example command under `pinocchio/cmd/examples/simple-redis-streaming-inference` that mirrors `geppetto/cmd/examples/simple-streaming-inference/main.go`, but publishes and consumes streaming LLM events over Redis. We’ll introduce a custom Glaze layer to configure Redis (address, stream, group, consumer) and wire Watermill’s Redis Publisher/Subscriber into Geppetto’s `EventRouter` and `WatermillSink`.

References:
- Base example to mirror: `geppetto/cmd/examples/simple-streaming-inference/main.go`
- Event model and router: `geppetto/pkg/events/chat-events.go`, `geppetto/pkg/events/event-router.go`
- Watermill sink: `geppetto/pkg/inference/middleware/sink_watermill.go`
- Glaze layers: `github.com/go-go-golems/glazed/pkg/cmds/layers` (run `glaze help custom-layer-tutorial` for more background)

## 1) Layout and files

- New directory: `pinocchio/cmd/examples/simple-redis-streaming-inference/`
- Files to add:
  - `main.go` — command wiring (mirrors the simple example: router + sink + engine + handlers + errgroup)
  - `redis_layer.go` — custom Glaze parameter layer definition for Redis

We keep the engine/tooling logic identical to the base example and only change transport from in-memory to Redis.

## 2) Add Watermill Redis dependency

We’ll use Watermill’s Redis Streams transport. Add to the `pinocchio` module:

```bash
go get github.com/ThreeDotsLabs/watermill-redisstream@latest && \
go get github.com/redis/go-redis/v9@latest
```

Notes:
- Package import is usually `github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream`.
- If you prefer a pub/sub flavor, you can also evaluate `watermill-redis`, but Streams offer consumer groups and durability.

## 3) Define a custom Glaze layer for Redis

Create `redis_layer.go` to define a typed layer with parameters. The layer will be attached to the command description alongside Geppetto’s layers, and we’ll initialize a typed settings struct from it at runtime.

Example struct (sketch):

```go
// pinocchio/cmd/examples/simple-redis-streaming-inference/redis_layer.go
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
    "redis", // slug
    layers.WithShort("Redis configuration for Watermill Redis Streams"),
    layers.WithFlags(
      parameters.NewParameterDefinition("redis-addr", parameters.ParameterTypeString, parameters.WithDefault("localhost:6379")),
      parameters.NewParameterDefinition("redis-stream", parameters.ParameterTypeString, parameters.WithDefault("chat")),
      parameters.NewParameterDefinition("redis-group", parameters.ParameterTypeString, parameters.WithDefault("chat-ui")),
      parameters.NewParameterDefinition("redis-consumer", parameters.ParameterTypeString, parameters.WithDefault("ui-1")),
    ),
  )
}
```

At runtime we’ll initialize a `RedisSettings` from the `"redis"` layer with `parsedLayers.InitializeStruct("redis", &redis)`.

Tip: See `glaze help custom-layer-tutorial` for more patterns (including struct-tag based layer generation). Either approach is fine.

## 4) Wire EventRouter with Redis Publisher/Subscriber

In `main.go`, mirror the base example, but construct `EventRouter` with Redis-backed publisher and subscriber using the `redisstream` package.

Sketch (focus on transport wiring):

```go
package main

import (
  "context"
  "io"
  "github.com/ThreeDotsLabs/watermill"
  "github.com/ThreeDotsLabs/watermill/message"
  rstream "github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
  "github.com/redis/go-redis/v9"
  "github.com/go-go-golems/geppetto/pkg/events"
  "github.com/go-go-golems/geppetto/pkg/inference/engine"
  "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
  "github.com/go-go-golems/geppetto/pkg/inference/middleware"
  geppettolayers "github.com/go-go-golems/geppetto/pkg/layers"
  "github.com/go-go-golems/glazed/pkg/cli"
  "github.com/go-go-golems/glazed/pkg/cmds"
  "github.com/go-go-golems/glazed/pkg/cmds/layers"
  // ... logging, help, parameters, errgroup, yaml if needed
)

type SimpleRedisStreamingInferenceSettings struct {
  // reuse fields from simple-streaming example (prompt, output-format, with-metadata, verbose, etc.)
  Prompt       string `glazed.parameter:"prompt"`
  OutputFormat string `glazed.parameter:"output-format"`
  WithMetadata bool   `glazed.parameter:"with-metadata"`
  FullOutput   bool   `glazed.parameter:"full-output"`
  Verbose      bool   `glazed.parameter:"verbose"`

  Redis RedisSettings // from our custom layer
}

func newRedisRouter(redisCfg RedisSettings, verbose bool) (*events.EventRouter, error) {
  // Go-redis client
  client := redis.NewClient(&redis.Options{ Addr: redisCfg.Addr })

  // Marshaler for Watermill Redis Streams
  marshaler := rstream.DefaultMarshaler{}
  logger := watermill.NopLogger{}

  pub, err := rstream.NewPublisher(rstream.PublisherConfig{
    Client:    client,
    Marshaler: marshaler,
    Stream:    redisCfg.Stream,
  }, logger)
  if err != nil { return nil, err }

  sub, err := rstream.NewSubscriber(rstream.SubscriberConfig{
    Client:        client,
    Marshaler:     marshaler,
    ConsumerGroup: redisCfg.Group,
    Consumer:      redisCfg.Consumer,
  }, logger)
  if err != nil { return nil, err }

  // Build EventRouter with Redis transports
  opts := []events.EventRouterOption{
    events.WithPublisher(pub),
    events.WithSubscriber(sub),
  }
  if verbose { opts = append(opts, events.WithVerbose(true)) }
  return events.NewEventRouter(opts...)
}

func (c *SimpleRedisStreamingInferenceCommand) RunIntoWriter(ctx context.Context, parsed *layers.ParsedLayers, w io.Writer) error {
  s := &SimpleRedisStreamingInferenceSettings{}
  if err := parsed.InitializeStruct(layers.DefaultSlug, s); err != nil { return err }
  if err := parsed.InitializeStruct("redis", &s.Redis); err != nil { return err }

  router, err := newRedisRouter(s.Redis, s.Verbose)
  if err != nil { return err }
  defer router.Close()

  // sink publishes to topic "chat" using the Redis publisher configured on router
  sink := middleware.NewWatermillSink(router.Publisher, "chat")

  // printer handler, same as base example
  // router.AddHandler("chat", "chat", events.StepPrinterFunc("", w)) or structured printer

  // engine with sink
  eng, err := factory.NewEngineFromParsedLayers(parsed, engine.WithSink(sink))
  if err != nil { return err }

  // build a Turn and run router + inference in parallel (errgroup)
  // identical to simple-streaming-inference/main.go
  return nil
}
```

Important:
- Topic remains `"chat"` to align with existing handlers and UIs.
- For production, ensure the Redis stream and consumer group exist or let the subscriber create them (depending on library behavior). For first runs, you may need to create the consumer group with `XGROUP CREATE`.

## 5) Mirror the base example structure

Follow `geppetto/cmd/examples/simple-streaming-inference/main.go` for:

- Command description and flags (`Prompt`, `OutputFormat`, `WithMetadata`, `FullOutput`, `Verbose`, `log-level`).
- Geppetto layers: `geppettolayers.CreateGeppettoLayers()`.
- Add the Redis layer to the command description with `cmds.WithLayersList(append(geLayers, redisLayer)...)`.
- In `RunIntoWriter`:
  - Initialize both default and redis layers into `SimpleRedisStreamingInferenceSettings`.
  - Build router (Redis-backed), add printer handler(s).
  - Create sink and engine.
  - Build a seed `turns.Turn` from prompt.
  - Start router and engine inference concurrently using `errgroup`, wait for completion, then print the final turn.

## 6) Handlers and printing

Add at least one handler to render the stream to stdout:

```go
if s.OutputFormat == "" {
  router.AddHandler("chat", "chat", events.StepPrinterFunc("", w))
} else {
  printer := events.NewStructuredPrinter(w, events.PrinterOptions{
    Format:          events.PrinterFormat(s.OutputFormat),
    Name:            "",
    IncludeMetadata: s.WithMetadata,
    Full:            s.FullOutput,
  })
  router.AddHandler("chat", "chat", printer)
}
```

You can add additional handlers later (e.g., logging tool calls, persisting events), the same way the `simple-chat-agent` does.

## 7) Running it

Prerequisites:
- Redis running locally (default `localhost:6379`).
- A valid Pinocchio/Geppetto provider profile (via `clay.InitViper("pinocchio", root)`).

Build and run:

```bash
go run ./pinocchio/cmd/examples/simple-redis-streaming-inference \
  --prompt "Hello via Redis" \
  --output-format text \
  --redis-addr localhost:6379 \
  --redis-stream chat \
  --redis-group chat-ui \
  --redis-consumer ui-1
```

You should see streaming deltas and a final completion, identical to the in-memory example, but transported through Redis Streams.

## 8) Producer-only / Consumer-only modes (optional)

Because Watermill separates publisher and subscriber, you can split the example into two processes:

- Producer: Only sets `WithPublisher` and uses the sink to publish (no router.Run).
- Consumer: Only sets `WithSubscriber` and runs the router handlers (no engine). This is useful for UIs (TUI or web) that display events from a remote engine.

## 9) Troubleshooting

- No events received
  - Verify the Redis stream and consumer group exist (or allow the library to create them). Check `XINFO STREAM chat` and `XINFO GROUPS chat`.
  - Ensure topic matches (`"chat"`).

- Duplicate events
  - Avoid configuring both engine-level sinks and context sinks unless needed (see `simple-chat-agent.md`).

- Serialization errors
  - Ensure handlers parse with `events.NewEventFromJson(msg.Payload)`.

## 10) Next steps

- Add a second example that runs as a consumer-only TUI (subscribe via Redis, render via Bobatea timeline).
- Add a minimal web server that forwards Redis events to WebSocket clients (see the Redis design doc `01-designing-a-redis-based-chat-web-agent.md`).

