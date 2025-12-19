---
Title: 'turnsdatalint: relaxing const-only to typed-key enforcement (allow conversions/variables)'
Ticket: 001-FIX-KEY-TAG-LINTING
Status: active
Topics:
    - lint
    - go-analysis
    - turnsdatalint
DocType: analysis
Intent: long-term
Owners:
    - manuel
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2025-12-18T18:00:00-05:00
---

# turnsdatalint: relaxing const-only to typed-key enforcement

## Executive Summary

The current `turnsdatalint` analyzer is **too strict**: it rejects typed conversions (`turns.TurnDataKey("foo")`), typed variables, and typed parameters—even though these are all type-safe and don't cause the "string drift" problem the linter was designed to prevent.

**The real goal** is simpler: **prevent raw string literals** like `t.Data["foo"]` while allowing normal Go patterns with typed keys.

This document proposes **relaxing the analyzer** to accept:
- ✅ Const keys: `t.Data[turns.DataKeyFoo]`
- ✅ Typed conversions: `t.Data[turns.TurnDataKey("foo")]`
- ✅ Typed variables: `key := turns.DataKeyFoo; t.Data[key]`
- ✅ Typed parameters: `func set(key TurnDataKey) { t.Data[key] = v }`

While still rejecting:
- ❌ Raw string literals: `t.Data["foo"]`
- ❌ Untyped variables: `s := "foo"; t.Data[s]`

## Current behavior (too strict)

From `geppetto/pkg/analysis/turnsdatalint/analyzer.go`, the analyzer only accepts `types.Const` keys:

```go
func isAllowedConstKey(pass *analysis.Pass, e ast.Expr, ...) bool {
    switch t := e.(type) {
    case *ast.Ident:
        return objIsAllowedConst(pass.TypesInfo.ObjectOf(t), ...)
    case *ast.SelectorExpr:
        return objIsAllowedConst(pass.TypesInfo.ObjectOf(t.Sel), ...)
    default:
        return false  // ← rejects CallExpr (conversions), vars, params
    }
}

func objIsAllowedConst(obj types.Object, ...) bool {
    c, ok := obj.(*types.Const)  // ← requires const identity
    if !ok {
        return false
    }
    // ... check type matches
}
```

This means:
- `turns.TurnDataKey("foo")` → **CallExpr** → rejected
- `key := turns.DataKeyFoo; t.Data[key]` → `key` is a **Var**, not a **Const** → rejected
- `func set(key TurnDataKey) { t.Data[key] = v }` → `key` is a **Var** (parameter) → rejected

## Why this is too strict

The original motivation (from `geppetto/pkg/doc/topics/12-turnsdatalint.md`) was:

> **prevents subtle drift where different packages use different string keys for the same concept**

The problem was **raw string literals** causing drift:
- Package A: `t.Data["tool_registry"]`
- Package B: `t.Data["toolRegistry"]`
- Package C: `t.Data["tools"]`

The strict const-only rule prevents this, but it also prevents **legitimate typed patterns**:

```go
// Legitimate: typed conversion (no drift possible)
t.Data[turns.TurnDataKey("custom_key")]  // ← still type-safe

// Legitimate: helper function with typed parameter
func SetData(t *Turn, key TurnDataKey, value any) {
    t.Data[key] = value  // ← key is typed, no string drift possible
}
```

## Current violations in Pinocchio (under strict rule)

As of diary Step 11, `make geppetto-lint` reports 3 violations:

### 1. `tool_loop_backend.go:54` - Typed conversion in loop

```go
func (b *ToolLoopBackend) WithInitialTurnData(data map[string]any) *ToolLoopBackend {
    for k, v := range data {
        b.Turn.Data[turns.TurnDataKey(k)] = v  // ← typed conversion
    }
    return b
}
```

**What it does**: Accepts a convenience `map[string]any` and converts keys.
**Why it's flagged**: `turns.TurnDataKey(k)` is a conversion, not a const.
**Is it a real problem?**: No—the conversion ensures type safety, and call sites control the keys.

### 2. `tool_loop_backend.go:68` - Typed parameter

```go
func (b *ToolLoopBackend) SetInitialTurnData(key turns.TurnDataKey, value interface{}) *ToolLoopBackend {
    b.Turn.Data[key] = value  // ← typed parameter
    return b
}
```

**What it does**: Helper method to set one Turn.Data entry.
**Why it's flagged**: `key` is a parameter (variable), not a const.
**Is it a real problem?**: No—call sites must provide a `TurnDataKey`, preventing string drift.

### 3. `middleware.go:114` - Local const (may need verification)

```go
const DataKeySQLiteDSN turns.TurnDataKey = "sqlite_dsn"

func middleware(...) {
    if v, ok := t.Data[DataKeySQLiteDSN].(string); ok {  // ← local const
        dsn = v
    }
}
```

**What it does**: Uses a package-local const.
**Why it might be flagged**: If the analyzer checks package path, a local const may not match.
**Is it a real problem?**: No—it's a properly typed const.

## What actually causes "string drift" (the real problem)

**Bad** (causes drift):
```go
t.Data["tool_registry"]  // ← raw string literal, untyped
```

**Good** (all type-safe):
```go
t.Data[turns.DataKeyToolRegistry]           // ← const
t.Data[turns.TurnDataKey("tool_registry")]  // ← typed conversion
key := turns.DataKeyToolRegistry; t.Data[key]  // ← typed variable
```

The typed variants **cannot cause drift** because:
1. The type system enforces `TurnDataKey`, not `string`
2. IDE/refactoring tools track typed identifiers
3. Grep for `TurnDataKey` finds all usage sites

## Proposed new rule: "typed keys" instead of "const-only keys"

### Rule

For Turn/Block maps with typed keys (`TurnDataKey`, `TurnMetadataKey`, `BlockMetadataKey`, `RunMetadataKey`):

**Allowed**:
- Const keys: `t.Data[turns.DataKeyFoo]`
- Typed conversions: `t.Data[turns.TurnDataKey("foo")]`
- Typed variables: `key := turns.DataKeyFoo; t.Data[key]`
- Typed parameters: `func set(key TurnDataKey) { t.Data[key] = v }`

**Rejected**:
- Raw string literals: `t.Data["foo"]`
- Untyped string variables: `s := "foo"; t.Data[s]`

### Implementation in analyzer

Update `isAllowedConstKey` in `geppetto/pkg/analysis/turnsdatalint/analyzer.go`:

```go
func isAllowedTypedKey(pass *analysis.Pass, e ast.Expr, wantPkgPath, wantName string) bool {
    e = unwrapParens(e)
    
    // Get the type of the expression
    tv, ok := pass.TypesInfo.Types[e]
    if !ok {
        return false
    }
    
    // Check if it's the correct named type (TurnDataKey, etc.)
    named, ok := tv.Type.(*types.Named)
    if !ok {
        return false
    }
    
    if wantName != "" && named.Obj().Name() != wantName {
        return false
    }
    
    if wantPkgPath != "" && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() != wantPkgPath {
        return false
    }
    
    // Accept any expression that has the correct type
    // This includes: consts, vars, params, conversions
    return true
}
```

**Key change**: Check `pass.TypesInfo.Types[e]` (type of the expression) instead of requiring `types.Const` identity.

### Special case: Block.Payload (map[string]any)

For `Block.Payload` (which is `map[string]any`, not typed), keep the stricter rule but allow const strings:

**Allowed**:
- Const string: `b.Payload[turns.PayloadKeyText]` where `PayloadKeyText = "text"` is a const

**Rejected**:
- String literals: `b.Payload["text"]`
- String variables: `s := "text"; b.Payload[s]`

Implementation:

```go
func isAllowedPayloadKey(pass *analysis.Pass, e ast.Expr) bool {
    e = unwrapParens(e)
    
    // Get the type of the expression
    tv, ok := pass.TypesInfo.Types[e]
    if !ok {
        return false
    }
    
    // Must be string type
    basic, ok := tv.Type.Underlying().(*types.Basic)
    if !ok || basic.Kind() != types.String {
        return false
    }
    
    // But must be a const, not a variable or literal
    switch t := e.(type) {
    case *ast.Ident:
        return objIsConst(pass.TypesInfo.ObjectOf(t))
    case *ast.SelectorExpr:
        return objIsConst(pass.TypesInfo.ObjectOf(t.Sel))
    default:
        return false  // Rejects string literals and conversions
    }
}

func objIsConst(obj types.Object) bool {
    _, ok := obj.(*types.Const)
    return ok
}
```

## Impact on existing violations

With the **relaxed rule**, all 3 current Pinocchio violations become **valid**:

### 1. `tool_loop_backend.go:54` - Typed conversion

```go
b.Turn.Data[turns.TurnDataKey(k)] = v
```

✅ **Passes**: Expression type is `turns.TurnDataKey`

### 2. `tool_loop_backend.go:68` - Typed parameter

```go
b.Turn.Data[key] = value  // key is TurnDataKey parameter
```

✅ **Passes**: Expression type is `turns.TurnDataKey`

### 3. `middleware.go:114` - Local typed const

```go
const DataKeySQLiteDSN turns.TurnDataKey = "sqlite_dsn"
t.Data[DataKeySQLiteDSN]
```

✅ **Passes**: Expression type is `turns.TurnDataKey`

## What still gets caught (the actual bad patterns)

### Raw string literals

```go
t.Data["raw_string"]  // ← REJECTED: untyped string literal
```

### Untyped string variables

```go
s := "raw_string"
t.Data[s]  // ← REJECTED: s is type string, not TurnDataKey
```

### Untyped conversions

```go
t.Data[TurnDataKey(someStringVar)]  // ← REJECTED if someStringVar is untyped string
```

But this is correct behavior—we **want** to catch these because they bypass the type system.

## Benefits of the relaxed rule

1. **Preserves the core safety**: raw strings are still caught
2. **Allows normal Go patterns**:
   - Helper functions with typed parameters
   - Loops over typed collections
   - Typed conversions for dynamic keys
3. **No need for workarounds**:
   - No allowlist of specific functions
   - No suppression directives
   - No "set data directly" verbosity at call sites
4. **Better ergonomics**: developers can write natural code with typed keys

## Implementation checklist

To implement this in geppetto:

- [ ] Update `geppetto/pkg/analysis/turnsdatalint/analyzer.go`:
  - Rename `isAllowedConstKey` → `isAllowedTypedKey`
  - Check `pass.TypesInfo.Types[e]` instead of requiring `types.Const`
  - For Block.Payload, keep const-only (since it's `map[string]any`)
- [ ] Update tests in `geppetto/pkg/analysis/turnsdatalint/analyzer_test.go`:
  - Add test cases for typed conversions (should pass)
  - Add test cases for typed variables (should pass)
  - Add test cases for typed parameters (should pass)
  - Keep test cases for raw strings (should fail)
- [ ] Update `geppetto/pkg/doc/topics/12-turnsdatalint.md`:
  - Document the new "typed keys" rule
  - Update examples to show conversions/variables are allowed
- [ ] Remove the `isInsideAllowedHelperFunction` allowlist (no longer needed)
- [ ] Test in geppetto: `make linttool`
- [ ] Test in pinocchio: `make geppetto-lint-build && make geppetto-lint`

## Recommendation

**Implement the relaxed rule in geppetto**, then:

1. **Pinocchio side**: No code changes needed—current violations become valid
2. **Geppetto side**: Update analyzer + tests + docs (one focused PR)
3. **All downstream repos**: Benefit from more ergonomic linting

This is the right long-term fix because:
- The strict "const-only" rule was solving a problem (raw strings) by over-constraining (rejecting typed keys)
- The relaxed "typed-only" rule solves the same problem with better ergonomics
- Type safety is the right boundary (not const-vs-var)

## Alternative: Keep strict rule, use workarounds in Pinocchio

If we decide **not** to relax the analyzer, then Pinocchio must:
- Remove helper methods (`WithInitialTurnData`, etc.)
- Set Turn.Data directly at all call sites using const keys only
- Accept the verbosity as the cost of strict enforcement

But this is **not recommended** because:
- It makes helper functions impossible to write
- It doesn't improve safety (typed keys are already safe)
- It creates friction for downstream repos adopting Geppetto

## Testing the relaxed rule

Once the analyzer is updated, verify:

```bash
# In geppetto:
make linttool  # Should pass with typed conversions/vars/params

# In pinocchio:
make geppetto-lint-build
make geppetto-lint  # Should show 0 violations

# Verify raw strings are still caught:
# Add a test case: t.Data["raw"] → should fail
```

## Files to update (in geppetto)

1. `pkg/analysis/turnsdatalint/analyzer.go`:
   - Lines 142-169: Replace `isAllowedConstKey` logic with type-based check
   - Lines 274-300: Remove `isInsideAllowedHelperFunction` (no longer needed)

2. `pkg/analysis/turnsdatalint/analyzer_test.go`:
   - Add positive test cases for conversions/vars/params

3. `pkg/doc/topics/12-turnsdatalint.md`:
   - Update "Allowed examples" section
   - Document that typed conversions/vars/params are allowed

4. `cmd/geppetto-lint/main.go`:
   - No changes needed

## Example code patterns (after relaxed rule)

### Pattern 1: Typed conversion for dynamic keys

```go
// Valid: conversion ensures type safety
func enableTool(t *Turn, toolKey string) {
    t.Data[turns.TurnDataKey(toolKey)] = true
}
```

### Pattern 2: Helper with typed parameter

```go
// Valid: parameter is typed, no string drift possible
func SetTurnData(t *Turn, key turns.TurnDataKey, value any) {
    if t.Data == nil {
        t.Data = map[turns.TurnDataKey]interface{}{}
    }
    t.Data[key] = value
}
```

### Pattern 3: Typed variable

```go
// Valid: variable is typed
key := turns.DataKeyToolRegistry
if reg, ok := t.Data[key].(ToolRegistry); ok {
    // use reg
}
```

### Anti-pattern: Raw string (still caught)

```go
// Invalid: raw string literal
t.Data["tool_registry"]  // ← analyzer rejects

// Invalid: untyped string variable
s := "tool_registry"
t.Data[s]  // ← analyzer rejects (s is string, not TurnDataKey)
```

## Migration path for other repos

Once geppetto is updated:

1. **No action needed** for repos using typed patterns (conversions/vars/params)
2. **Action required** for repos with raw string literals:
   - Find: `grep -r 'Data\["' --include="*.go"`
   - Fix: Replace with typed conversions or const references
3. **Benefit**: Natural Go patterns work, safety is preserved

## References

- Current analyzer: `geppetto/pkg/analysis/turnsdatalint/analyzer.go`
- Documentation: `geppetto/pkg/doc/topics/12-turnsdatalint.md`
- Geppetto's suppression analysis: `geppetto/ttmp/2025/12/18/002-FIX-GLAZED-INIT--fix-glazed-init/analysis/01-turnsdatalint-inline-suppression-ignore-comments-options.md`
