## Web UI Template Conversion

Converted web UI templates from Go's html/template to templ for better type safety and maintainability.

- Converted base.html to base.templ
- Converted messages.html to components/chat.templ and components/events.templ
- Updated server.go to use templ components
- Improved type safety with proper WebMessage handling
- Removed template dependency from SSEClient
- Made all templ functions public with uppercase names for better package API 