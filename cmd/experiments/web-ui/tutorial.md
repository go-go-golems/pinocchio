# Building a Server-Sent Events Backend for Streaming Steps

This tutorial explains how to build a web server that streams events from a step using Server-Sent Events (SSE) and HTMX.

## Overview

We'll build a web server that:
1. Handles client connections using SSE
2. Manages client sessions with unique IDs
3. Converts step events to HTML
4. Uses HTMX for dynamic updates
5. Supports session persistence through URLs

## Project Structure

```
cmd/experiments/web-ui/
├── main.go           # Server implementation
├── templates/        # HTML templates
│   ├── index.html   # Main page template
│   └── events.html  # Event templates
└── tutorial.md      # This tutorial
```

## Core Components

### 1. SSE Client Management

We use a `SSEClient` struct to manage individual client connections:

```go
type SSEClient struct {
    ID           string
    MessageChan  chan string
    DisconnectCh chan struct{}
    DroppedMsgs  int64
}
```

Each client has:
- A unique ID
- A buffered message channel
- A disconnect channel for cleanup
- A counter for dropped messages

### 2. Step Management

The `StepInstance` struct manages running steps:

```go
type StepInstance struct {
    Step   *chat.EchoStep
    Topic  string
    Cancel context.CancelFunc
}
```

This allows us to:
- Track active steps
- Associate them with clients
- Cancel them when needed

### 3. Server State

The `Server` struct maintains the application state:

```go
type Server struct {
    tmpl       *template.Template
    router     *events.EventRouter
    clients    map[string]*SSEClient
    steps      map[string]*StepInstance
    clientsMux sync.RWMutex
    stepsMux   sync.RWMutex
    logger     zerolog.Logger
    metrics    struct {
        TotalDroppedMsgs int64
    }
}
```

## Key Features

### 1. Event to HTML Conversion

Events from steps are converted to HTML using templates:

```go
func EventToHTML(tmpl *template.Template, e chat.Event) (string, error) {
    // Convert different event types to HTML snippets
    // using templates defined in events.html
}
```

### 2. Client Session Management

Sessions are managed through client IDs in URLs:
- New clients get a UUID
- Client ID is pushed to URL using HTMX
- Sessions can be reconnected using URL parameters

### 3. SSE Connection Handling

The `/events` endpoint handles SSE connections:
1. Sets appropriate headers
2. Maintains connection with heartbeats
3. Streams events as they arrive
4. Handles multiline messages properly

### 4. Step Event Routing

Events flow through the system:
1. Step publishes to Watermill topic
2. Router forwards to handler
3. Handler converts to HTML
4. HTML is sent to client's message channel
5. SSE connection streams to browser

## Detailed Implementation Guide

### 1. Event Router Setup

The event router is based on Watermill, a Go library for working with message streams:

```go
// Create event router with verbose logging
router, err := events.NewEventRouter(events.WithVerbose(true))
if err != nil {
    logger.Fatal().Err(err).Msg("Failed to create event router")
}
defer router.Close()

// Start router in background
go func() {
    logger.Info().Msg("Starting router")
    if err := router.Run(context.Background()); err != nil {
        logger.Fatal().Err(err).Msg("Router failed")
    }
    defer func() {
        router.Close()
        logger.Info().Msg("Router stopped")
    }()
}()
```

The router:
- Manages message routing between publishers and subscribers
- Runs in a separate goroutine
- Handles graceful shutdown
- Provides verbose logging for debugging

### 2. Step Creation and Configuration

Steps are created and configured in multiple stages:

```go
func (s *Server) CreateStep(clientID string) error {
    s.stepsMux.Lock()
    defer s.stepsMux.Unlock()

    // 1. Cancel existing step if any
    if instance, ok := s.steps[clientID]; ok {
        instance.Cancel()
        delete(s.steps, clientID)
        s.logger.Info().Str("client_id", clientID).Msg("Cancelled existing step")
    }

    // 2. Create new step with proper initialization
    step := chat.NewEchoStep()
    step.TimePerCharacter = 50 * time.Millisecond

    // 3. Setup unique topic for this client
    topic := fmt.Sprintf("chat-%s", clientID)
    
    // 4. Configure step to publish events
    if err := step.AddPublishedTopic(s.router.Publisher, topic); err != nil {
        s.logger.Error().Err(err).
            Str("client_id", clientID).
            Msg("Failed to setup event publishing")
        return fmt.Errorf("error setting up event publishing: %w", err)
    }

    // 5. Store step instance
    s.steps[clientID] = &StepInstance{
        Step:  step,
        Topic: topic,
    }

    return nil
}
```

### 3. Event Handler Registration

Each client gets a dedicated event handler:

```go
// Add handler for this client's events
s.router.AddHandler(
    topic,      // Handler name (must be unique)
    topic,      // Topic to subscribe to
    func(msg *message.Message) error {
        // Handler implementation
    },
)
```

The handler processes events in several stages:

1. **Event Parsing**:
```go
// Parse raw message into event
e, err := chat.NewEventFromJson(msg.Payload)
if err != nil {
    s.logger.Error().Err(err).
        Str("client_id", clientID).
        Str("message_id", msg.UUID).
        Str("payload", string(msg.Payload)).
        Msg("Failed to parse event")
    return err
}
```

2. **Event Type Handling**:
```go
// Convert event to HTML based on type
func EventToHTML(tmpl *template.Template, e chat.Event) (string, error) {
    data := EventTemplateData{
        Timestamp: time.Now().Format("15:04:05"),
    }

    var templateName string
    switch e_ := e.(type) {
    case *chat.EventPartialCompletionStart:
        templateName = "event-start"

    case *chat.EventPartialCompletion:
        templateName = "event-partial"
        data.Completion = e_.Completion

    case *chat.EventFinal:
        templateName = "event-final"
        data.Text = e_.Text

    // ... handle other event types ...
    }

    var buf bytes.Buffer
    if err := tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
        return "", fmt.Errorf("error executing template: %w", err)
    }

    return buf.String(), nil
}
```

### 4. Step Execution

Steps are started with a conversation:

```go
func (s *Server) StartStep(clientID string) error {
    s.stepsMux.Lock()
    instance, ok := s.steps[clientID]
    s.stepsMux.Unlock()

    if !ok {
        return fmt.Errorf("no step found for client %s", clientID)
    }

    // 1. Create cancellable context
    ctx, cancel := context.WithCancel(context.Background())
    instance.Cancel = cancel

    // 2. Create conversation
    msgs := []*conversation.Message{
        conversation.NewChatMessage(conversation.RoleSystem, 
            "You are a helpful assistant."),
        conversation.NewChatMessage(conversation.RoleUser, 
            "Hello! Please tell me a short story about a robot."),
    }

    // 3. Start step with conversation
    result, err := instance.Step.Start(ctx, msgs)
    if err != nil {
        return fmt.Errorf("error starting step: %w", err)
    }

    // 4. Process results in background
    go func() {
        resultCount := 0
        for result := range result.GetChannel() {
            resultCount++
            if result.Error() != nil {
                s.logger.Error().
                    Err(result.Error()).
                    Str("client_id", clientID).
                    Int("result_count", resultCount).
                    Msg("Error in step result")
                continue
            }
            // Results are handled by the router through events
        }
        s.logger.Info().
            Str("client_id", clientID).
            Int("total_results", resultCount).
            Msg("Step completed")
    }()

    return nil
}
```

### 5. Event Templates

Events are rendered using HTML templates. Here's an example of the event templates:

```html
{{define "event-start"}}
<div class="event">
    <span class="timestamp">{{.Timestamp}}</span>
    <div class="content">Starting chat...</div>
</div>
{{end}}

{{define "event-partial"}}
<div class="event">
    <span class="timestamp">{{.Timestamp}}</span>
    <div class="content">{{.Completion}}</div>
</div>
{{end}}

{{define "event-final"}}
<div class="event">
    <span class="timestamp">{{.Timestamp}}</span>
    <div class="content">{{.Text}}</div>
</div>
{{end}}

{{define "event-tool-call"}}
<div class="event">
    <span class="timestamp">{{.Timestamp}}</span>
    <div class="tool-call">
        <strong>Tool Call:</strong> {{.Name}}
        <pre>{{.Input}}</pre>
    </div>
</div>
{{end}}
```

### 6. Event Flow

Here's the complete flow of an event through the system:

1. **Step Generation**:
   ```go
   // Step generates event
   e.subscriptionManager.PublishBlind(NewPartialCompletionEvent(
       metadata, stepMetadata, string(c_), msg.Text[:idx+1]))
   ```

2. **Router Processing**:
   ```go
   // Watermill router receives message
   // Routes to registered handler based on topic
   ```

3. **Handler Processing**:
   ```go
   // Handler receives message
   // Parses JSON into Event
   // Converts Event to HTML using template
   ```

4. **Client Delivery**:
   ```go
   // Send HTML to client's message channel
   if !client.TrySend(html) {
       // Handle dropped message
   }
   ```

5. **SSE Streaming**:
   ```go
   // SSE endpoint reads from message channel
   case msg, ok := <-client.MessageChan:
       if !ok {
           return
       }
       // Handle multiline messages
       lines := strings.Split(msg, "\n")
       fmt.Fprintf(w, "event: message\n")
       for _, line := range lines {
           fmt.Fprintf(w, "data: %s\n", line)
       }
       fmt.Fprintf(w, "\n")
       flusher.Flush()
   ```

### 7. Error Handling and Recovery

The system handles errors at multiple levels:

1. **Step Level**:
   - Context cancellation
   - Result channel errors
   - Publishing errors

2. **Router Level**:
   - Message parsing errors
   - Handler errors
   - Topic errors

3. **Client Level**:
   - Connection drops
   - Buffer overflows
   - Timeouts

4. **Template Level**:
   - Template execution errors
   - Invalid event types
   - Missing data

Each level includes:
- Error logging
- Resource cleanup
- Client notification
- Graceful degradation

## Implementation Steps

### 1. Setup Project Structure

```bash
mkdir -p cmd/experiments/web-ui/templates
touch cmd/experiments/web-ui/main.go
touch cmd/experiments/web-ui/templates/index.html
touch cmd/experiments/web-ui/templates/events.html
```

### 2. Create HTML Templates

The main template (`index.html`) uses HTMX:
- Conditional rendering based on client ID
- SSE connection setup
- URL-based session handling

### 3. Implement Server

Key endpoints:
1. `/` - Serves main page
2. `/start` - Creates new chat session
3. `/events` - Handles SSE connections

### 4. Event Handling

1. Create step instance
2. Setup event routing
3. Convert events to HTML
4. Stream to client

## Best Practices

1. **Error Handling**
   - Graceful error recovery
   - Detailed error logging
   - Client-friendly error messages

2. **Resource Management**
   - Buffered channels
   - Proper cleanup
   - Connection timeouts

3. **Logging**
   - Structured logging with zerolog
   - Different log levels
   - Contextual information

4. **Concurrency**
   - Mutex protection
   - Atomic counters
   - Safe channel operations

## HTMX Integration

1. **SSE Setup**
```html
<div hx-ext="sse" 
     sse-connect="/events?client_id={{.ClientID}}" 
     sse-swap="message">
</div>
```

2. **URL Handling**
```html
<button hx-get="/start"
        hx-target="#chat-container"
        hx-push-url="true"
        hx-swap="innerHTML">
    Start New Chat
</button>
```

3. **Server Response**
```go
w.Header().Set("HX-Push-Url", fmt.Sprintf("/?client_id=%s", clientID))
```

## Testing

1. **Start Server**
```bash
go run cmd/experiments/web-ui/main.go
```

2. **Access UI**
- Open http://localhost:8080
- Click "Start New Chat"
- Watch events stream in

3. **Test Session Persistence**
- Copy URL with client ID
- Open in new tab
- Should reconnect to same session

## Common Issues

1. **Message Buffering**
   - Use appropriate buffer sizes
   - Handle dropped messages
   - Monitor queue lengths

2. **Connection Management**
   - Handle disconnects gracefully
   - Send periodic heartbeats
   - Clean up resources

3. **Event Routing**
   - Ensure unique topics
   - Handle all event types
   - Validate event data

## Conclusion

This implementation provides:
- Real-time event streaming
- Session persistence
- Clean separation of concerns
- Robust error handling
- Comprehensive logging

The combination of SSE and HTMX offers a simple yet powerful way to stream events to the browser without WebSocket complexity.

## Next Steps

1. Add authentication
2. Implement rate limiting
3. Add message persistence
4. Enhance error recovery
5. Add metrics collection 