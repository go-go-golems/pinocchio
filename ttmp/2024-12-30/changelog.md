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