# Simple Inference Example

A complete pinocchio command that demonstrates the Engine-first architecture with glazed command patterns.

## Usage

```bash
# Build the application (uses pinocchio's go.mod)
go build

# Basic inference
./simple-inference simple-chat "Hello, how are you?"

# With different profile
PINOCCHIO_PROFILE=gpt-4 ./simple-inference simple-chat "What's the capital of France?"

# CLI mode (single inference)
./simple-inference simple-inference --cli-mode "Tell me a joke"

# With logging middleware
./simple-inference simple-inference --with-logging "Explain quantum computing"

# Debug mode (show parsed layers)
./simple-inference simple-inference --debug
```

## What it demonstrates

- **Glazed Command Pattern**: Follows the established pinocchio command structure with proper layer parsing
- **Engine-first Architecture**: Uses `inference.NewEngineFromParsedLayers()` for engine creation
- **Middleware System**: Shows how to add logging middleware to engines
- **Layer Integration**: Proper use of `helpers.ParseGeppettoLayers()` for configuration
- **Profile System**: Integration with pinocchio's profile-based configuration
- **Multiple Modes**: CLI mode vs interactive chat modes

## Architecture Highlights

### Engine Creation with Parsed Layers

```go
// Create base engine from parsed layers (includes profile configuration)
baseEngine, err := inference.NewEngineFromParsedLayers(geppettoParsedLayers)

// Add middleware for additional functionality
var middlewares []inference.Middleware
if s.WithLogging {
    middlewares = append(middlewares, loggingMiddleware)
}

// Wrap engine with middleware
engine := inference.NewEngineWithMiddleware(baseEngine, middlewares...)
```

### Command Structure

The example follows the established pattern:
1. **YAML Definition**: `command.yaml` defines the base pinocchio chat command  
2. **Wrapper Command**: `SimpleInferenceCommand` implements `WriterCommand` interface
3. **Layer Parsing**: Uses `helpers.ParseGeppettoLayers()` for proper configuration
4. **Engine Integration**: Creates engine from parsed layers with optional middleware

## Flags

- `--pinocchio-profile`: Set the pinocchio profile (default: "4o-mini")
- `--debug`: Show parsed layers configuration  
- `--cli-mode`: Single inference mode without chat UI
- `--with-logging`: Enable logging middleware to see request/response flow

## Environment Variables

All standard pinocchio environment variables are supported:
- `PINOCCHIO_PROFILE`: Set the default profile
- `OPENAI_API_KEY`: OpenAI API key (required for GPT models)
- `ANTHROPIC_API_KEY`: Anthropic API key (required for Claude models)
