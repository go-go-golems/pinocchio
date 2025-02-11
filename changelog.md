# Changelog

## Streamlined Interactive and Chat Mode
Improved the interactive and chat mode flow by consolidating router management and using RunMode to determine whether to run an initial blocking step. This change makes the interactive mode a special case of chat mode where we first run a blocking step before potentially entering chat mode.

- Refactored runInteractive to use runChat with RunMode check
- Consolidated router management in runChat
- Added proper nil checks for UISettings 

# Refactor eval command structure

Improved the organization of the eval command by separating concerns and using modern templating:

- Split eval command into separate eval/ package
- Created new serve/ package for web view functionality
- Converted HTML template to use templ language
- Added proper Go module structure 

# Improved web view template type safety

Added a new RenderableTestOutput struct to improve type safety in templates:

- Created intermediate struct for web view data
- Removed interface{} type assertions from templates
- Added proper string conversion for numeric IDs
- Improved error handling for missing fields 

# Fixed templ script handling

Fixed script handling in web view template:

- Fixed hx-target attribute to use proper Go string concatenation
- Added proper templ script handling for onclick events
- Improved type safety in JavaScript event handling 

# Profile Management Commands

Added a new `profiles` command group to manage pinocchio profiles:
- `list`: List all available profiles
- `get`: Get profile settings (entire profile, layer, or specific key)
- `set`: Set a profile setting
- `delete`: Delete a profile or specific setting
- `edit`: Edit the profiles file in your default editor
- `init`: Initialize a new profiles file with documentation and examples

The profiles command allows managing the profiles.yaml configuration file which contains layer-specific settings for different profiles.
The init command creates a new profiles file with helpful documentation and examples of the file format.

# Previous Changes

// ... existing code ... 