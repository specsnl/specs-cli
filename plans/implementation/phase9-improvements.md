# Phase 9 improvements — Smarter variable prompting

**Status: implemented** (2026-04-26)

Two deficiencies were identified after the initial phase 9 implementation:

1. **Nested-conditional over-prompting** — a variable whose condition references another
   conditional variable (e.g. `PgPort` gated on `DbType == "pg"`, where `DbType` is itself
   conditional) is evaluated before `DbType` has been answered. The evaluation uses the
   schema default, which may not match the user's eventual answer, causing unnecessary prompts.

2. **Unreferenced-variable prompting** — schema variables that appear in `project.yaml` but
   are not referenced in any template file or computed expression are always prompted, even
   though their value can never affect the output.

---

## Fix 1 — Iterative pass-2 prompting

### Root cause

`promptContext` currently computes `neededCondKeys` in one shot after pass 1:

```go
for _, k := range condKeys {
    if conds[k].Eval(ctx) {          // ctx["DbType"] is still the default here
        neededCondKeys = append(neededCondKeys, k)
    }
}
runPromptPass(ctx, schema, neededCondKeys, provided)
```

If `DbType` is itself a conditional variable (prompted during the same pass 2),
`ctx["DbType"]` at evaluation time is the schema default, not the user's answer.

### Solution

Replace the fixed pass 2 with a dependency-ordered loop. A conditional key is *ready to
evaluate* when every variable it references (via `Cond.Keys()`) is already resolved. A
variable is resolved when it has a final value — either from pass 1, from `--arg`/`--values`,
or from a previous iteration of the loop.

Keys that are not in the schema are implicitly resolved: they cannot be prompted, so their
current `ctx` value is already final.

```
resolved := alwaysKeys ∪ providedKeys

loop:
    ready = { k ∈ remaining | ∀ gateKey ∈ conds[k].Keys(): gateKey ∈ resolved ∨ gateKey ∉ schema }
    if ready is empty → break

    toPrompt = { k ∈ ready | conds[k].Eval(ctx) == true }
    runPromptPass(ctx, schema, sorted(toPrompt), provided)

    resolved ∪= ready      // mark both prompted and skipped keys as resolved
    remaining -= ready
```

Each iteration produces one huh form. For most templates this is still two forms total
(one always-pass, one conditional-pass). Deeply nested `eq`/`ne` chains may produce more,
but the number of iterations is bounded by the depth of the condition dependency graph.

### Changes

**`pkg/cmd/template_use.go`**

Replace the current pass-2 block in `promptContext` with the loop described above.
No signature changes are needed — `conds` and `schema` already carry all required information.

Helper: a `schemaKeySet` built once from `schema` so the `gateKey ∉ schema` check is O(1).

### Tests

| Test | Scenario | Expected |
|------|----------|----------|
| `TestPromptContext_NestedEq_SkipInner` | `DbType` gated on `UseDB`; `PgPort` gated on `UseDB AND DbType=="pg"`; defaults `UseDB=true, DbType="mysql"` | `PgPort` not prompted |
| `TestPromptContext_NestedEq_IncludeInner` | same; defaults `UseDB=true, DbType="pg"` | `PgPort` prompted |
| `TestPromptContext_ThreeLevel` | three-level dependency chain; innermost skipped | correct |

These are unit tests on `promptContext` directly, using a stub `runPromptPass` or
`--use-defaults` equivalents in command tests.

---

## Fix 2 — Skip unreferenced schema variables

### Root cause

`AnalyzeConditionals` discards the `always` map after cleanup and only returns `Conditionals`.
`promptContext` therefore cannot distinguish "not referenced anywhere" from "referenced
unconditionally": both are absent from `conds`.

### Solution

#### Part A — expose the referenced set from `AnalyzeConditionals`

Change the signature to return a second value: the set of all variable names referenced
anywhere in the template file tree (union of `always` and the final `conds` keys).

```go
func AnalyzeConditionals(
    templateRoot string,
    userCtx      map[string]any,
    funcMap      texttemplate.FuncMap,
) (Conditionals, map[string]bool, error)
```

The returned `map[string]bool` is built as:

```go
referenced := make(map[string]bool)
for k := range always          { referenced[k] = true }
for k := range conds           { referenced[k] = true }   // final conds, post-cleanup
```

#### Part B — scan computed expressions

`ComputedDefs` contains template expressions (`"[[ .Name ]]_prod"`) that can reference
schema variables. A variable appearing only in a computed expression — but not in any
template file — would be missed by Part A alone.

In `Get()`, after calling `AnalyzeConditionals`, scan each computed expression body with
the existing `extractRefs` helper and union the results into the referenced set:

```go
for _, expr := range computedDefs {
    for _, key := range extractRefs(expr, funcMap) {
        if _, inSchema := userCtx[key]; inSchema {
            referenced[key] = true
        }
    }
}
```

Only schema keys are relevant: computed-to-computed references do not represent promptable
variables.

#### Part C — store on `Template` and filter in `promptContext`

Add a `Referenced map[string]bool` field to `Template`. Populate it in `Get()` with the
union from Parts A and B.

Pass it to `promptContext` (new parameter) and apply the filter when building `alwaysKeys`
and `condKeys`:

```go
for _, k := range sortedKeys(schema) {
    if !referenced[k] { continue }    // never used anywhere — skip entirely
    ...
}
```

### Changes

| File | Change |
|------|--------|
| `pkg/template/analysis.go` | `AnalyzeConditionals` returns `(Conditionals, map[string]bool, error)` |
| `pkg/template/template.go` | `Template` gains `Referenced map[string]bool`; `Get()` populates it |
| `pkg/cmd/template_use.go` | `promptContext` receives and applies `referenced`; call site passes `tmpl.Referenced` |

### Tests

| Test | Scenario | Expected |
|------|----------|----------|
| `TestAnalysis_UnusedVar_NotReferenced` | schema has `Unused: ""`, no template references it | `Unused` absent from referenced set |
| `TestAnalysis_ComputedRef_IsReferenced` | schema has `Name: ""`, only referenced in computed `Upper: "[[ toUpper .Name ]]"`, no template refs | `Name` in referenced set |
| `TestTemplateUse_UnusedVarNotPrompted` | schema has `Unused: ""`, template never uses it, `--use-defaults` absent | command succeeds; `Unused` was never shown |

---

## Known limitations (out of scope for this fix)

- **Computed chains + conditional template use**: If a schema variable `A` is only referenced
  in computed expression `B`, and `B` itself is only referenced inside a conditional block in
  template files, `A` will still be always prompted. Propagating conditional information
  through the computed-value dependency graph would require a more complex analysis pass.

- **Hook scripts**: Hook scripts receive all context variables as env vars. Variables that
  are only meaningful to hooks (not in template files or computed expressions) will be
  incorrectly skipped. This is acceptable: hook-only variables should also appear somewhere
  in the template to keep the schema self-documenting.

---

## Context merge order (updated)

```
1. LoadUserContext(templateRoot)
   ↓
2. AnalyzeConditionals(templateRoot, userCtx, funcMap)
        → Conditionals map
        → Referenced set (template files)
   ↓
3. extractRefs over ComputedDefs → union into Referenced
   ↓
4. Merge --values file
   ↓
5. Merge --arg flags
   ↓
6. Prompt — Pass 1: always-needed variables that are in Referenced
   ↓
7. Prompt — Iterative pass: for each dependency-ordered batch of conditional
            variables whose gate keys are resolved, evaluate condition and
            prompt the true ones
   ↓
8. ApplyComputed(ctx, defs)
   ↓
9. Execute template + run hooks
```
