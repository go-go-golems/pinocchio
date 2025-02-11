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

# Enhanced Profiles List Command

Enhanced the profiles list command to show full profile contents by default, with a new --concise flag to only show profile names.

- Added full profile content display in profiles list command
- Added --concise/-c flag to show only profile names
- Updated ListProfiles method to return both names and full content

# Improved Profiles Editor with Ordered Maps

Enhanced the profiles editor to use ordered maps and type aliases for better clarity and consistency:

- Added type aliases for profile, layer, and setting names/values
- Used ordered maps to preserve the order of layers and settings
- Updated profiles commands to handle ordered maps
- Improved code readability with semantic type names

# Improved Profiles Editor Loading

Modified the profiles editor to load at runtime instead of construction time:

- Profiles editor now loads when commands are executed
- Improved error handling and logging
- Fixed directory creation in edit command
- Consistent with config editor loading behavior

# Added Config Editing Commands

Added a new set of commands for managing the Viper configuration file:

- `config list`: List all configuration keys and values (with --concise flag)
- `config get`: Get a specific configuration value
- `config set`: Set a configuration value
- `config delete`: Delete a configuration value
- `config edit`: Edit the configuration file in your default editor

The config editor uses Viper to manage the configuration file and provides a simple interface for viewing and modifying settings.

# Improved Config Editor Loading

Modified the config editor to load at runtime instead of construction time:

- Config editor now loads when commands are executed
- Always uses the current config file from Viper
- Improved handling of default config path
- Fixed directory creation in edit command

# Added Profile Duplication Command

Added a new command to duplicate existing profiles:

- Added `profiles duplicate` command to copy profiles
- Implemented deep copying of YAML nodes to preserve structure
- Maintains all settings, layers, and comments from source profile
- Added validation to prevent overwriting existing profiles

# Previous Changes

// ... existing code ... 