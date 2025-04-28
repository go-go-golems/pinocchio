---
Title: Using catter to gather source code for LLMs
Slug: catter
Short: Process and analyze source code for LLM context preparation and token analysis
Topics:
- catter
- llm
Commands:
- catter print
- catter stats
Flags:
- max-file-size
- max-total-size
- include
- exclude
- match-filename
- match-path
- exclude-dirs
- delimiter
- archive-file
- archive-prefix
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

The `pinocchio catter` command is a tool for preparing and analyzing source code for Large Language Model (LLM) contexts. It offers two main subcommands: `print` for outputting and processing file contents, and `stats` for analyzing codebase statistics.

## File Filtering System

The catter command provides a powerful and flexible file filtering system that helps you precisely control which files are processed.

### Default Filters

By default, catter excludes common binary and non-text files:

1. Binary File Extensions:
   - Images: `.png`, `.jpg`, `.jpeg`, `.gif`, `.bmp`, `.tiff`, `.webp`
   - Audio: `.mp3`, `.wav`, `.ogg`, `.flac`
   - Video: `.mp4`, `.avi`, `.mov`, `.wmv`
   - Archives: `.zip`, `.tar`, `.gz`, `.rar`
   - Executables: `.exe`, `.dll`, `.so`, `.dylib`
   - Documents: `.pdf`, `.doc`, `.docx`, `.xls`, `.xlsx`
   - Data: `.bin`, `.dat`, `.db`, `.sqlite`
   - Fonts: `.woff`, `.ttf`, `.eot`, `.svg`, `.woff2`
   - Lock files: `.lock`

2. Excluded Directories:
   - Version Control: `.git`, `.svn`
   - Dependencies: `node_modules`, `vendor`
   - IDE/Editor: `.history`, `.idea`, `.vscode`
   - Build: `build`, `dist`, `sorbet`
   - Documentation: `.yardoc`

3. Excluded Filenames (regex patterns):
   - `.*-lock\.json$`
   - `go\.sum$`
   - `yarn\.lock$`
   - `package-lock\.json$`

These defaults can be disabled using the `--disable-default-filters` flag.

### Filter Configuration Options

#### Extension-based Filtering
The following examples use these flags:
- `-i, --include`: Specify file extensions to include
- `-e, --exclude`: Specify file extensions to exclude

```bash
# Include only specific extensions
pinocchio catter print -i .go,.js,.py

# Exclude specific extensions
pinocchio catter print -e .test.js,.spec.py

# Combine include and exclude
pinocchio catter print -i .go,.js -e .test.js
```

#### Pattern Matching
Flags used:
- `-f, --match-filename`: Match filenames using regex patterns
- `-p, --match-path`: Match file paths using regex patterns
- `--exclude-match-filename`: Exclude files matching regex patterns
- `--exclude-match-path`: Exclude paths matching regex patterns

```bash
# Match test files
pinocchio catter print -f "^test_.*\.py$"

# Match multiple patterns
pinocchio catter print -f "^main.*" -f "^app.*"

# Match specific directories while excluding tests
pinocchio catter print -p "src/models/" --exclude-match-path "internal/testing/"
```

#### Directory Exclusion
Using `-x, --exclude-dirs` to specify directories to skip:

```bash
# Exclude multiple directories
pinocchio catter print -x tests,docs,examples,vendor
```

#### Size and Binary Filtering
Flags:
- `--max-file-size`: Maximum size for individual files (bytes)
- `--filter-binary`: Control binary file filtering

```bash
# Set maximum file size and include binary files
pinocchio catter print --max-file-size 500000 --filter-binary=false
```

#### GitIgnore Integration

```bash
# Use repository's .gitignore rules (default)
pinocchio catter print .

# Disable .gitignore rules
pinocchio catter print --disable-gitignore .
```

### YAML Configuration

Create a `.catter-filter.yaml` file to define reusable filter profiles:

```yaml
profiles:
  go-only:
    include-exts: [.go]
    exclude-dirs: [vendor, test]
    exclude-match-filenames: [".*_test\\.go$"]
    max-file-size: 1048576  # 1MB
    filter-binary-files: true

  docs:
    include-exts: [.md, .rst, .txt]
    match-paths: ["docs/", "README"]
    exclude-dirs: [node_modules, vendor]

  tests:
    match-filenames: ["^test_", "_test\\.go$"]
    exclude-dirs: [vendor]
```

Use profiles with:
```bash
pinocchio catter print --filter-profile go-only .
```

### Debugging Filters

Use the verbose flag to see which files are being included or excluded:

```bash
pinocchio catter print --verbose .
```

Print current filter configuration:
```bash
pinocchio catter print --print-filters
```

### Filter Precedence

Filters are applied in the following order:

1. GitIgnore rules (unless disabled)
2. File size limits
3. Default exclusions (unless disabled)
4. Extension includes
5. Extension excludes
6. Filename pattern matches
7. Path pattern matches
8. Directory exclusions
9. Binary file filtering

A file must pass all applicable filters to be included in the output.

### Best Practices

1. **Start Broad, Then Narrow**
   ```bash
   # Start with extension filtering
   pinocchio catter print --include .py .
   
   # Add specific patterns
   pinocchio catter print --include .py --match-filename "^(?!test_).*\.py$"
   ```

2. **Use Multiple Filter Types**
   ```bash
   # Combine different filter types for precision
   pinocchio catter print \
     --include .go \
     --exclude-dirs vendor,test \
     --match-path "src/" \
     --exclude-match-filename "_test\.go$"
   ```

3. **Profile-based Workflow**
   - Create profiles for common tasks
   - Use environment variables for profile selection
   - Share profiles across team members

4. **Performance Considerations**
   - Start with directory exclusions for large codebases
   - Use file size limits for large files
   - Enable binary filtering to avoid processing non-text files

## Common Use Cases

### 1. Preparing Code for LLM Prompts
Flags used:
- `-d, --delimiter`: Output format for text (markdown, xml, simple, begin-end)
- `-s, --stats`: Statistics detail level (overview, dir, full)

```bash
# Get Python files with context headers (text output)
pinocchio catter print -i .py -x tests/ -d markdown src/

# Process specific files with token statistics
pinocchio catter stats -s full main.go utils.go config.go
```

### 2. Archiving Filtered Files
Flags used:
- `-a, --archive-file`: Output archive file path (e.g., `output.zip`, `codebase.tar.gz`)
- `--archive-prefix`: Directory prefix within the archive (e.g., `my-project/`)

```bash
# Archive all .go files (excluding vendor) into a zip file
pinocchio catter print -i .go -x vendor -a go_files.zip .

# Archive .py and .js files into a tar.gz, placing them under a 'src' prefix
pinocchio catter print -i .py,.js --archive-prefix src/ -a source_archive.tar.gz .

# Archive files matching a path pattern into a zip file
pinocchio catter print -p "internal/api/" -a api_files.zip .
```

### 3. Token-Aware Processing
Flags:
- `--max-tokens`: Limit total tokens processed (applies to text output and archive content)
- `--max-lines`: Limit lines per file (applies to text output and archive content)
- `--glazed`: Enable structured output (text output only)

```bash
# Limit tokens while getting detailed stats (text output)
pinocchio catter print --max-tokens 4000 --max-lines 100 --glazed src/

# Limit tokens when creating an archive
pinocchio catter print --max-tokens 10000 -i .go -a limited_go.zip .

# Get structured stats output
pinocchio catter stats --glazed -s full . | glazed format -f json
```

## Command Reference

### Print Command

`pinocchio catter print [flags] <paths...>`

Main flags:
- `--max-file-size`: Limit individual file sizes (default: 1MB)
- `--max-total-size`: Limit total processed size (default: 10MB)
- `-i, --include`: File extensions to include (e.g., .go,.js)
- `-e, --exclude`: File extensions to exclude
- `-d, --delimiter`: Output format for text output (default, xml, markdown, simple, begin-end)
- `--max-lines`: Maximum lines per file (applies to text and archive)
- `--max-tokens`: Maximum tokens per file (applies to text and archive)
- `-a, --archive-file`: Path to output archive file. Format (zip or tar.gz/.tgz) inferred from extension. If set, text output flags (`-d`, `--glazed`) are ignored.
- `--archive-prefix`: Directory prefix to add within the archive (e.g., `myproject/`). Used only with `--archive-file`.
- `--glazed`: Enable structured output (ignored if `--archive-file` is used)

Filtering options:
- `-f, --match-filename`: Regex patterns for filenames
- `-p, --match-path`: Regex patterns for file paths
- `-x, --exclude-dirs`: Directories to exclude
- `--disable-gitignore`: Ignore .gitignore rules
- `--print-filters`: Print the resolved filter configuration and exit.
- `--filter-yaml`: Path to a YAML file with filter profiles.
- `--filter-profile`: Name of a filter profile to use from YAML.
- `--disable-default-filters`: Disable built-in default filters.

### Stats Command

`pinocchio catter stats [flags] <paths...>`

Main flags:
- `-s, --stats`: Statistics detail level (overview, dir, full)
- `--glazed`: Enable structured output (default: true)

The stats command provides:
- Total token counts
- File and directory statistics
- Extension-based analysis
- Line counts and file sizes

## Advanced Usage

### 1. Using YAML Configuration

Create a `.catter-filter.yaml` file for persistent settings:

```yaml
profiles:
  python-only:
    include-exts: [.py]
    exclude-dirs: [venv, __pycache__]
  api-docs:
    match-paths: ["api/", "docs/"]
    include-exts: [.md, .rst]
```

Use profiles:
```bash
pinocchio catter print --filter-profile python-only .
```

### 2. Structured Output

Generate machine-readable output (only for text output modes):

```bash
# Get JSON-formatted stats
pinocchio catter stats --glazed -s full . | glazed format -f json

# Process text output with other tools
pinocchio catter print --glazed src/ | glazed filter --col Content
```

### 3. Context-Aware Processing

Maintain code context with delimiters:

```bash
# XML format for structured parsing / claude 
pinocchio catter print -d xml src/

# Markdown format separator
pinocchio catter print -d markdown --include .md,.rst docs/
```

### 4. Gitignore Integration

Respect repository settings:

```bash
# Use repository's .gitignore
pinocchio catter print .

# Override gitignore rules
pinocchio catter print --disable-gitignore .
```

## Tips and Best Practices

1. **Token Optimization**
   - Use `--max-tokens` to stay within API limits
   - Combine with `--max-lines` for reasonable chunk sizes
   - Use stats command to analyze token usage patterns

2. **Filtering Strategy**
   - Start with broad filters and refine
   - Use `--print-filters` to verify configuration
   - Combine path and filename patterns for precision

3. **Output Management**
   - Choose appropriate delimiters for your use case
   - Use structured output for automation
   - Consider file size limits for large codebases

4. **Configuration Management**
   - Use YAML profiles for repeated tasks
   - Set CATTER_PROFILE environment variable
   - Create project-specific filter configurations

## Error Handling

Common error scenarios and solutions:

1. **Size Limits**
   - "maximum total size limit reached": Increase `--max-total-size`
   - "maximum tokens limit reached": Adjust `--max-tokens`

2. **Filter Issues**
   - No files processed: Check filter patterns
   - Unexpected files: Verify .gitignore settings

3. **Performance**
   - Large directories: Use specific paths
   - Memory usage: Set appropriate size limits

## Integration Examples

### 1. With LLM Tools

```bash
# Prepare code for OpenAI API
pinocchio catter print --max-tokens 4000 -d markdown src/ > context.md

# Generate documentation
pinocchio catter print --include .go --exclude-dirs vendor/ . | pinocchio code professional --context - "Generate documentation"
```

### 2. With Development Workflows

```bash
# Code review preparation
pinocchio catter print --match-path "changed/" -d markdown > review.md

# Documentation updates
pinocchio catter stats -s full . > codebase-metrics.json
```
