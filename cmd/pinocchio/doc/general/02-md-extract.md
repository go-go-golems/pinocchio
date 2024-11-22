---
Title: MD-Extract - Extract content from markdown files
Slug: md-extract
Short: Extract and process different types of blocks from markdown files
Topics:
- markdown
- extraction
- code blocks
- text blocks
- pinocchio
Commands:
- md-extract
- pinocchio helpers
Flags:
- output
- with-quotes
- allowed-languages
- blocks
- file
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Overview

MD-Extract is a utility for extracting and processing content from markdown files. It can extract code blocks, normal text blocks, or both, with various output formats and filtering options.

## Basic Usage

Extract code blocks from a markdown file:
```bash
pinocchio helpers md-extract --file input.md
```

Extract all blocks (code and normal text):
```bash
pinocchio helpers md-extract --file input.md --blocks all
```

Process markdown from stdin:
```bash
cat input.md | pinocchio helpers md-extract 
```

## Command Flags

### Output Format

`--output <string>`
- Controls the output format
- Choices: "concatenated", "list", "yaml"
- Default: "concatenated"
- Examples:
  ```bash
  # Simple concatenation of blocks
  pinocchio helpers md-extract --file input.md --output concatenated

  # Detailed list with block metadata
  pinocchio helpers md-extract --file input.md --output list

  # YAML format for structured processing
  pinocchio helpers md-extract --file input.md --output yaml
  ```

### Block Selection

`--blocks <string>`
- Controls which types of blocks to extract
- Choices: "all", "normal", "code"
- Default: "code"
- Examples:
  ```bash
  # Extract all blocks
  pinocchio helpers md-extract --file input.md --blocks all

  # Extract only normal text blocks
  pinocchio helpers md-extract --file input.md --blocks normal

  # Extract only code blocks (default)
  pinocchio helpers md-extract --file input.md --blocks code
  ```

### Code Block Options

`--with-quotes <bool>`
- Include markdown code block quotes in output
- Default: false
- Example:
  ```bash
  pinocchio helpers md-extract --file input.md --with-quotes
  ```

`--allowed-languages <list>`
- Filter code blocks by programming language
- Optional: if not specified, all languages are included
- Example:
  ```bash
  pinocchio helpers md-extract --file input.md --allowed-languages python,go
  ```

### Input Source

`--file <string>`
- Input file path (use "-" for stdin)
- Default: "-"
- Examples:
  ```bash
  # Read from file
  pinocchio helpers md-extract --file README.md

  # Read from stdin
  cat README.md | pinocchio helpers md-extract --file -
  ```

## Output Formats

### Concatenated (Default)

Outputs blocks one after another:
```
// For code blocks with --with-quotes
```python
def hello():
    print("Hello")
```

// For code blocks without --with-quotes
def hello():
    print("Hello")

// For normal text blocks
This is a normal text block.
```

### List

Outputs blocks with metadata:
```
Language: python
```python
def hello():
    print("Hello")
```
---
Type: normal
This is a normal text block.
---
```

### YAML

Outputs blocks in structured YAML format:
```yaml
- type: code
  language: python
  content: |
    def hello():
        print("Hello")
- type: normal
  content: This is a normal text block.
```

## Common Use Cases

1. **Extract Code Examples**
   ```bash
   # Extract Python code examples from documentation
   pinocchio helpers md-extract --file docs.md --allowed-languages python
   ```

2. **Documentation Processing**
   ```bash
   # Extract all content in structured format
   pinocchio helpers md-extract --file README.md --blocks all --output yaml
   ```

3. **Code Block Collection**
   ```bash
   # Collect all code blocks with language markers
   pinocchio helpers md-extract --file tutorial.md --with-quotes
   ```

4. **Text Content Extraction**
   ```bash
   # Extract only text content
   pinocchio helpers md-extract --file article.md --blocks normal
   ```
