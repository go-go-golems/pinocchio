---
Title: Diary
Ticket: FIX-GLAZED-FLAGS
Status: active
Topics:
    - bug
    - flags
    - initialization
DocType: reference
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: cmd/pinocchio/main.go
      Note: |-
        Fixed help flag handling and restored early logging init by pre-parsing only logging flags (plus an early-flagset debug dump)
        Early logging now pre-parses only logging flags and can print early-parsed values via --debug-early-flagset
ExternalSources: []
Summary: ""
LastUpdated: 2025-12-18T19:02:35.115245501-05:00
---



# Diary

## Goal

Fix the flag parsing issue in pinocchio where `--help` flag causes "Could not parse flags: pflag: help requested" error. The problem is in the initialization sequence where `rootCmd.ParseFlags()` is called manually before cobra can properly handle help requests.

## Step 1: Identify the Root Cause

The issue was in `pinocchio/cmd/pinocchio/main.go` where `rootCmd.ParseFlags(os.Args[1:])` was being called manually on line 61. When `--help` or `-h` is passed, `ParseFlags` returns an error "pflag: help requested" because cobra hasn't been set up to handle help yet. This error was being caught and printed, causing the program to exit before cobra could display the help text.

**Commit (code):** N/A (this initial approach was superseded by Step 2)

### What I did
- Reproduced the issue: `go run ./cmd/pinocchio code unix hello --help` failed with "Could not parse flags: pflag: help requested"
- Examined the code flow in `main.go`:
  - Line 61: `rootCmd.ParseFlags(os.Args[1:])` called manually
  - Line 66: `logging.InitLoggerFromCobra(rootCmd)` called (duplicate, already in `PersistentPreRunE`)
  - Line 109: `rootCmd.Execute()` called, which would handle help naturally
- Reviewed how other commands handle this (e.g., `clay/examples/simple/logging_layer_example.go`) - they don't call `ParseFlags` manually
- Checked `glazed/pkg/cmds/logging/init.go` to understand `InitLoggerFromCobra` behavior

### Why
The manual `ParseFlags` call was added to initialize logging early (before command initialization), but it breaks cobra's natural help handling. Cobra expects to handle `--help` during `Execute()`, not during manual flag parsing.

### What worked
- Identified that the issue is specific to manual `ParseFlags` call
- Confirmed that logging initialization happens in `PersistentPreRunE` (line 52-54), so the duplicate call on line 66 is unnecessary
- Found that checking for `--help` before parsing is a valid workaround

### What didn't work
- Initially considered removing `ParseFlags` entirely, but realized we need early logging initialization for the `run-command` path and command loading

### What I learned
- Cobra's `ParseFlags` returns an error when `--help` is requested, but this is expected behavior - cobra handles help during `Execute()`
- The logging layer is initialized in `PersistentPreRunE`, which runs after cobra parses flags but before command execution
- Manual flag parsing breaks cobra's help handling mechanism

### What was tricky to build
- Balancing early logging initialization (needed for command loading) with proper help handling
- Understanding that `ParseFlags` error on `--help` is expected, not a bug - we just need to skip manual parsing when help is requested

### What warrants a second pair of eyes
- The check for `--help` flag is a simple string comparison - verify this handles all help flag variations (`-h`, `--help`, `help` subcommand)
- Confirm that skipping early logging initialization when help is requested doesn't cause issues (logging will still be initialized in `PersistentPreRunE` if a command actually runs)
- Verify the `run-command` path still works correctly with this change

### What should be done in the future
- Consider refactoring to avoid manual `ParseFlags` entirely - perhaps initialize logging differently for the `run-command` path
- Add tests for help flag handling to prevent regressions
- Document the initialization sequence and why manual flag parsing is needed

### Code review instructions
- Start in `pinocchio/cmd/pinocchio/main.go`, lines 57-70
- Test: `go run ./cmd/pinocchio code unix hello --help` should show help, not error
- Test: `go run ./cmd/pinocchio --help` should show root help
- Test: `go run ./cmd/pinocchio code unix hello` (without --help) should work normally
- Verify logging still initializes correctly in both help and non-help paths

### Technical details

The fix adds a check for `--help` or `-h` flags before calling `ParseFlags`:

```go
// Check if --help or -h is requested before manually parsing flags
hasHelpFlag := false
for _, arg := range os.Args[1:] {
    if arg == "--help" || arg == "-h" {
        hasHelpFlag = true
        break
    }
}

// Only parse flags manually if help is not requested
if !hasHelpFlag {
    err = rootCmd.ParseFlags(os.Args[1:])
    // ... initialize logging early
}
```

This allows:
1. Help requests to be handled naturally by cobra during `Execute()`
2. Early logging initialization for non-help paths (needed for command loading)
3. Logging still initializes in `PersistentPreRunE` for actual command execution

### What I'd do differently next time
- Test help flag handling earlier in the development process
- Consider using cobra's built-in help handling more consistently rather than manual flag parsing

## Step 2: Restore correct early log-level parsing (and add an early-flagset dump)

After Step 1, `--help` worked again, but the output was noisy: help could print a flood of debug lines while Pinocchio loads YAML commands (“Loading command from file”). This happens because command loading runs **before** cobra executes `PersistentPreRunE`, so logging wasn’t initialized yet and zerolog’s default global level let debug through.

This step replaces the brittle “early `rootCmd.ParseFlags`” approach with a dedicated early logging initializer that **filters `os.Args` down to only logging flags** and parses them with a small `pflag.FlagSet`. That gives us correct log level during repository loading without interfering with cobra’s normal parsing during `Execute()`. It also adds a hidden `--debug-early-flagset` flag to print exactly what the early parse saw.

**Commit (code):** 4da6556 — "pinocchio: pre-parse logging flags before command loading"

### What I did
- Updated `pinocchio/cmd/pinocchio/main.go`:
  - Added `filterEarlyLoggingArgs(args []string) []string` to keep only logging flags.
  - Added `initEarlyLoggingFromArgs(args []string) error` using a dedicated `pflag.FlagSet` + `logging.InitLoggerFromSettings(...)`.
  - Registered a hidden cobra persistent flag `--debug-early-flagset` (so passing it doesn’t break cobra).
  - When `--debug-early-flagset` is set, print:
    - the filtered args used for early parsing
    - the resolved early logging values (`--log-level`, `--log-format`, logstash flags, etc.)

### Why
- Debug spam comes from glazed command loading code that logs at debug level during repository traversal.
- That code runs before cobra pre-run hooks, so we need logging initialized earlier than `PersistentPreRunE`.
- We still want cobra to be the source of truth for full flag parsing; early parsing should be “logging-only”.

### What worked
- Default help output is quiet again:
  - `go run ./cmd/pinocchio code unix hello --help` no longer prints the debug “Loading command from file” lines.
- The early-flagset dump shows exactly what got parsed:
  - `go run ./cmd/pinocchio --debug-early-flagset code unix hello --help --log-level warn`

### What didn't work
- N/A

### What I learned
- `pflag` can stop on unknown flags; filtering down to known logging flags is the simplest robust strategy when commands/flags are registered dynamically.

### What was tricky to build
- Making sure the debug-only flag doesn’t become an “unknown flag” error later (fixed by registering it on `rootCmd` and hiding it).
- Keeping defaults in sync with `logging.AddLoggingLayerToRootCommand`.

### What warrants a second pair of eyes
- Double-check the “known logging flags” list stays in sync with the logging layer over time.

### What should be done in the future
- Add a regression test/script that asserts help output doesn’t contain `Loading command from file` unless `--log-level debug` is explicitly set.

### Code review instructions
- Start in `pinocchio/cmd/pinocchio/main.go`:
  - `filterEarlyLoggingArgs`
  - `initEarlyLoggingFromArgs`
  - call site in `main()` before `initAllCommands`
- Validate manually:
  - `go run ./cmd/pinocchio code unix hello --help`
  - `go run ./cmd/pinocchio --debug-early-flagset code unix hello --help --log-level warn`

### Technical details
- Debug log spam source: `glazed/pkg/cmds/loaders/loaders.go` emits `log.Debug().Str("file", fileName).Msg("Loading command from file")`.
