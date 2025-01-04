# Add structured event printer with metadata support

Added a new structured printer implementation to support different output formats and metadata inclusion. This enables better debugging and monitoring of the AI conversation process.

- Created new PrinterOptions struct for configuring output format and metadata inclusion
- Added support for text, JSON, and YAML output formats
- Implemented metadata and step metadata printing options
- Structured the output to cleanly separate event type, content, and metadata
- Added comprehensive documentation with usage examples 

# Add structured printer configuration flags

Added new command line flags to control output format and metadata inclusion, along with non-interactive mode support.

- Added --output flag to select between text/json/yaml formats
- Added --with-metadata and --with-step-metadata flags for detailed event information
- Added --non-interactive flag to skip chat mode entirely
- Integrated structured printer with command line options 

# Add compact metadata printing format

Updated the structured printer to provide a more focused view of important metadata while maintaining full output capability.

- Added compact metadata format focusing on model, temperature, tokens, and completion status
- Added --full-output flag to access complete metadata when needed
- Improved start event to show input token usage
- Structured final event to show token usage and stop reason clearly 

# Refactor Settings Initialization in GeppettoCommand

Simplified the settings initialization flow by moving it into the setupInfrastructure method to reduce code complexity and improve maintainability.

- Removed separate initializeSettings method
- Integrated settings initialization directly into setupInfrastructure
- Updated RunIntoWriter to use settings from commandContext 

# Extract CommandContext into Standalone Type

Extracted commandContext into a standalone type with a builder pattern to improve modularity and reusability.

- Created CommandContextOption for flexible context configuration
- Added WithXXX option functions for each context field
- Implemented NewCommandContext and NewCommandContextFromLayers builders
- Removed dependency on GeppettoCommand for context creation 

# Refactor Command Context

Moved conversation management, step execution, and chat handling from GeppettoCommand to CommandContext for better separation of concerns and reusability.

- Moved `initializeConversationManager` to `CommandContext`
- Moved step execution logic to `CommandContext`
- Moved chat and UI handling to `CommandContext`
- Simplified `GeppettoCommand` to focus on configuration
- Added proper cleanup with `Close` method in `CommandContext` 

# Extract ConversationContext into Standalone Type

Created a new ConversationContext type to handle conversation initialization and management, improving modularity and separation of concerns.

- Created ConversationContext with builder pattern for flexible configuration
- Added support for system prompts, messages, variables, and images
- Moved template rendering and conversation initialization logic
- Added proper autosave settings handling
- Simplified CommandContext by delegating conversation management 

# Extract Conversation Context Creation

Refactored conversation context creation in GeppettoCommand to improve code organization and reusability.

- Extracted CreateConversationContext method from CreateCommandContextFromSettings
- Updated CreateCommandContextFromSettings to use the new method 

# Refactor Conversation Context Creation

Improved CreateConversationContext method to be more focused and reusable.

- Removed helperSettings dependency from CreateConversationContext
- Moved image paths and autosave settings handling to caller
- Added support for combining default and custom conversation context options 