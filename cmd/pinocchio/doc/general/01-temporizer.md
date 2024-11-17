---
Title: Temporizer - Creating temporary files
Slug: temporizer
Short: Using temporizer for managing temporary context files
Topics:
- temporizer
- context
- files
- pinocchio
- prompto
Commands:
- temporizer
- pinocchio
- prompto
Flags:
- name
- file-prefix
- prefix
- suffix
- context
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Overview

Temporizer creates and manages temporary files from stdin, primarily used for passing context to other commands. It handles file cleanup automatically and provides options for file naming and content formatting.

## Basic Usage

Create a temporary file from stdin:
```bash
echo "data" | temporizer
```

The command outputs the path to the created temporary file.

## Command Flags

### Optional Flags

`--name, -n <string>`
- Sets identifier in filename
- Default: "default"
- Example: `--name context-1` → `/tmp/temporizer-context-1-<timestamp>`

`--file-prefix, -p <string>`
- Sets prefix for filename
- Default: "temporizer"
- Example: `--file-prefix myapp` → `/tmp/myapp-<name>-<timestamp>`

`--prefix <string>`
- Adds content before stdin data
- Example: `--prefix "--- Context Start ---"`

`--suffix <string>`
- Adds content after stdin data
- Example: `--suffix "--- Context End ---"`

## Automatic Cleanup

Temporizer includes built-in garbage collection that:
- Maintains only the 10 most recent temporary files
- Deletes older files based on modification time
- Runs automatically before creating new files
- Requires read/write permissions on temp directory
- Reports deleted files to stderr

## Integration with Pinocchio

Temporizer is especially useful for providing context to LLM commands:

```bash
pinocchio code professional \
    --context $(ppp glazed/writing-help-entries) \
    --context $(git diff origin/main | temporizer)
```

Key components:
1. `ppp` (alias for `prompto get --print-path`) retrieves prompto document paths
2. `temporizer` captures dynamic content like git diffs
3. Multiple context sources can be combined via --context flags

## Common Usage Patterns

With git changes:
```bash
git diff | temporizer --name git-changes
```

With API responses:
```bash
curl api.example.com | temporizer \
  --name api-data \
  --prefix "// Retrieved at $(date)" \
  --suffix "// End of response"
```

Multiple context sources:
```bash
pinocchio code professional \
  --context $(prompto get background --print-path) \
  --context $(curl api.example.com/data | temporizer) \
  --prompt "Analyze this data"
```

## File Management Details

- Files stored in system temp directory
- Unique filenames include timestamps
- Automatic cleanup of older files
- Reports cleanup actions to stderr
- Non-zero exit code on errors

## Output

- Created file path printed to stdout
- Cleanup/deleted files reported to stderr
- Error messages sent to stderr

This combination of automatic management and flexible options makes temporizer ideal for handling dynamic context in command pipelines, especially with LLM tools like Pinocchio.