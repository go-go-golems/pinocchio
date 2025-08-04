# Tool Use Example

A complete pinocchio command that demonstrates tool calling middleware with a fake weather tool.

## Features

- **Real Toolbox**: Uses `geppetto/pkg/toolbox.RealToolbox` for function registration and execution
- **Tool Middleware**: Demonstrates the Engine middleware system with tool calling capabilities
- **Weather Tool**: Simple fake weather service that provides consistent mock data
- **Glazed Command Pattern**: Follows the established pinocchio command structure with proper layer parsing
- **Multiple Modes**: Supports both CLI and interactive chat modes

## Usage

### CLI Mode (Single Query)

```bash
# Build the tool
go build

# Basic weather query
./tool-use tool-use --cli-mode "What's the weather in San Francisco?"

# Compare multiple cities
./tool-use tool-use --cli-mode "Compare the weather in New York and London"

# Use with specific profile
PINOCCHIO_PROFILE=4o-mini ./tool-use tool-use --cli-mode "Is it raining in Seattle?"
```

### Interactive Mode

```bash
# Start interactive chat with tool capabilities
./tool-use tool-use "What's the weather like today?"

# Debug mode - see parsed layers
./tool-use tool-use --debug
```

### Available Commands

```bash
# Show help
./tool-use --help

# Show weather-chat command help  
./tool-use weather-chat --help

# Show tool-use wrapper command help
./tool-use tool-use --help
```

## How It Works

### Command Structure

1. **YAML Definition**: `command.yaml` defines the base pinocchio chat command
2. **Wrapper Command**: `ToolUseCommand` wraps the pinocchio command with additional flags
3. **Tool Registration**: Weather tool is registered with the real toolbox
4. **Middleware Integration**: Tool middleware is applied to the engine
5. **Layer Parsing**: Uses `helpers.ParseGeppettoLayers()` for proper configuration

### Tool Integration

```go
// Create toolbox and register tools
tb := toolbox.NewRealToolbox()
weatherTool := &WeatherTool{}
err = tb.RegisterTool("get_weather", weatherTool.GetWeather)

// Create tool middleware
toolConfig := inference.ToolConfig{
    MaxIterations: 5,
    Timeout:       30, // seconds
}
toolMiddleware := inference.NewToolMiddleware(tb, toolConfig)

// Wrap engine with middleware
engine := inference.NewEngineWithMiddleware(baseEngine, toolMiddleware)
```

### Weather Tool

The `WeatherTool` provides a `GetWeather(city string) (*WeatherResult, error)` function that:

- Returns consistent fake weather data based on city name
- Includes temperature, condition, humidity, wind speed, and description
- Has built-in "geographic" logic for realistic city-based weather patterns
- Handles error cases (empty city names)

## Flags

- `--pinocchio-profile`: Set the pinocchio profile (default: "4o-mini")
- `--debug`: Show parsed layers configuration
- `--cli-mode`: Single inference mode without chat UI

## Environment Variables

All standard pinocchio environment variables are supported:

- `PINOCCHIO_PROFILE`: Set the default profile
- `OPENAI_API_KEY`: OpenAI API key (required for GPT models)
- `ANTHROPIC_API_KEY`: Anthropic API key (required for Claude models)

## Example Queries

- "What's the weather in San Francisco?"
- "Compare the weather in New York and London"
- "Is it raining in Seattle?"
- "Tell me about the weather in Tokyo"
- "What's the temperature in Miami?"

The AI will automatically call the weather tool when weather information is requested and provide natural language responses based on the tool results.

## Architecture

This example demonstrates:

1. **Real Tool Integration**: Using reflection-based tool registration and execution
2. **Engine Middleware**: The functional middleware pattern for extending engine capabilities  
3. **Pinocchio Command Pattern**: Following established patterns for command structure and configuration
4. **Layer System**: Proper use of glazed layers for configuration management
5. **Profile System**: Integration with pinocchio's profile-based configuration
