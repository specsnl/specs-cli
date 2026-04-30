# Phase 9 — Conditional variable prompting

**Status: implemented** (2026-04-26)

## Goal

Before prompting the user, statically analyse the template's file tree to determine which
schema variables are only ever referenced inside conditional blocks. Variables that will
not be reached given the user's boolean answers are skipped entirely — no prompt, no
question. No changes to `project.yaml` syntax are required; template authors get this
behaviour automatically.

---

## Done criteria

- A variable referenced exclusively inside `[[if .Gate]]…[[end]]` is not prompted when
  the user answers `false` to `Gate`.
- A variable referenced exclusively inside an `[[else]]` branch is not prompted when the
  user answers `true` to `Gate`.
- Negation (`[[if not .Gate]]`) is treated correctly.
- Equality comparisons (`[[if eq .Var "value"]]`) gate variables on a specific string
  value of `Var`.
- `and`/`or` conditions gate variables on the conjunction/disjunction of sub-conditions.
- Nested `[[if]]` blocks produce a conjunction of all enclosing conditions.
- A variable referenced *both* inside and outside a conditional block is always prompted.
- Any condition the analyser cannot classify (unknown function, chained pipelines, etc.)
  falls back to "always prompt" — the safe default.
- Skipped variables retain their schema default value and remain present in the template
  context; computed values and file content templates can still reference them.
- All tests pass.

---

## File overview

```
pkg/
└── template/
    ├── cond.go          (new — Cond interface + all concrete types + eval)
    ├── analysis.go      (new — AnalyzeConditionals, AST walker, pipe parser)
    └── template.go      (modified — Conditionals field, call analysis in Get)
pkg/
└── cmd/
    └── template_use.go  (modified — two-pass promptContext)
```

---

## Design: condition expression tree

All condition types are represented as a small recursive interface. Each node knows how to
evaluate itself against a context map.

### `pkg/template/cond.go`

```go
package template

import "fmt"

// Cond is a boolean condition derived from a [[if …]] expression in a template file.
// All concrete types are unexported; callers interact only through Eval and Keys.
type Cond interface {
    // Eval returns true when this condition is satisfied by the given context.
    Eval(ctx map[string]any) bool
    // Keys returns all schema variable names referenced by this condition.
    Keys() []string
}

// condField is satisfied when the named key is truthy (bool true or non-empty string).
type condField struct{ key string }

// condNot negates a sub-condition.
type condNot struct{ sub Cond }

// condEq is satisfied when ctx[key] == val (string comparison via fmt.Sprint).
type condEq struct{ key string; val any }

// condNe is satisfied when ctx[key] != val.
type condNe struct{ key string; val any }

// condAnd is satisfied when ALL sub-conditions are satisfied.
type condAnd struct{ subs []Cond }

// condOr is satisfied when ANY sub-condition is satisfied.
type condOr struct{ subs []Cond }

func (c condField) Eval(ctx map[string]any) bool { return isTruthy(ctx[c.key]) }
func (c condNot)   Eval(ctx map[string]any) bool { return !c.sub.Eval(ctx) }
func (c condEq)    Eval(ctx map[string]any) bool { return fmt.Sprint(ctx[c.key]) == fmt.Sprint(c.val) }
func (c condNe)    Eval(ctx map[string]any) bool { return fmt.Sprint(ctx[c.key]) != fmt.Sprint(c.val) }
func (c condAnd)   Eval(ctx map[string]any) bool {
    for _, s := range c.subs {
        if !s.Eval(ctx) { return false }
    }
    return true
}
func (c condOr) Eval(ctx map[string]any) bool {
    for _, s := range c.subs {
        if s.Eval(ctx) { return true }
    }
    return false
}

func (c condField) Keys() []string { return []string{c.key} }
func (c condNot)   Keys() []string { return c.sub.Keys() }
func (c condEq)    Keys() []string { return []string{c.key} }
func (c condNe)    Keys() []string { return []string{c.key} }
func (c condAnd)   Keys() []string { return collectKeys(c.subs) }
func (c condOr)    Keys() []string { return collectKeys(c.subs) }

func collectKeys(subs []Cond) []string {
    var keys []string
    for _, s := range subs {
        keys = append(keys, s.Keys()...)
    }
    return keys
}

// isTruthy returns true for bool true or a non-empty string; false for everything else.
func isTruthy(v any) bool {
    switch val := v.(type) {
    case bool:   return val
    case string: return val != ""
    default:     return false
    }
}
```

---

## Design: AST analysis

### `pkg/template/analysis.go`

Two exported symbols: `AnalyzeConditionals` (called from `Get`) and the result type.

#### Data structures

```go
// conditionalEntry records the effective condition under which a schema variable
// is accessed. If a variable is accessed both conditionally and unconditionally
// in different files or branches, it is removed from the map (always prompt).
type conditionals map[string]Cond   // varName → condition that must be true
```

#### `AnalyzeConditionals`

```go
// AnalyzeConditionals walks every file and every filename template under
// templateRoot/template/ and returns a conditionals map.
// A variable appears in the map only when it is accessed exclusively inside
// recognisable conditional blocks; otherwise it is absent (always prompt).
func AnalyzeConditionals(
    templateRoot string,
    userCtx map[string]any,
    funcMap texttemplate.FuncMap,
) (conditionals, error) {
    srcRoot := filepath.Join(templateRoot, specs.TemplateDirFile)

    // boolSchema: which schema keys are boolean (eligible as simple gates).
    boolSchema := make(map[string]bool, len(userCtx))
    for k, v := range userCtx {
        if _, ok := v.(bool); ok {
            boolSchema[k] = true
        }
    }

    always := make(map[string]bool)   // keys seen unconditionally
    conds  := make(conditionals)      // keys seen only conditionally (so far)

    err := filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        rel, _ := filepath.Rel(srcRoot, path)
        if rel == "." { return nil }

        // 1. Analyse the filename/path template itself.
        analyseExpr(rel, nil, funcMap, boolSchema, conds, always)

        // 2. For files, analyse the content (skip binaries and parse errors).
        if !d.IsDir() {
            data, err := os.ReadFile(path)
            if err != nil { return err }
            analyseExpr(string(data), nil, funcMap, boolSchema, conds, always)
        }
        return nil
    })
    if err != nil {
        return nil, err
    }

    // Remove any key that turned out to be unconditionally accessed.
    for k := range always {
        delete(conds, k)
    }
    return conds, nil
}
```

#### `analyseExpr` — parse one expression and walk its AST

```go
func analyseExpr(
    src string,
    outerGate Cond,          // conjunction of all enclosing if-gates (nil at top level)
    funcMap texttemplate.FuncMap,
    boolSchema map[string]bool,
    conds conditionals,
    always map[string]bool,
) {
    if !strings.Contains(src, "[[") { return }
    funcs := map[string]any(funcMap)
    tree, err := parse.New("").Parse(src, "[[", "]]", map[string]*parse.Tree{}, funcs)
    if err != nil || tree == nil || tree.Root == nil { return }
    walkNode(tree.Root, outerGate, funcMap, boolSchema, conds, always)
}
```

#### `walkNode` — recursive AST walker

```go
func walkNode(
    node parse.Node,
    gate Cond,               // effective condition at this point in the tree
    funcMap texttemplate.FuncMap,
    boolSchema map[string]bool,
    conds conditionals,
    always map[string]bool,
) {
    if node == nil { return }

    switch n := node.(type) {
    case *parse.ListNode:
        for _, child := range n.Nodes {
            walkNode(child, gate, funcMap, boolSchema, conds, always)
        }

    case *parse.IfNode:
        // Always walk the condition pipe at the current gate level
        // (gate variables are themselves referenced and must be always prompted).
        walkNode(n.Pipe, gate, funcMap, boolSchema, conds, always)

        innerCond, ok := parsePipeCond(n.Pipe)
        if ok {
            thenGate := andGates(gate, innerCond)
            elseGate := andGates(gate, condNot{innerCond})
            walkNode(n.List, thenGate, funcMap, boolSchema, conds, always)
            if n.ElseList != nil {
                walkNode(n.ElseList, elseGate, funcMap, boolSchema, conds, always)
            }
        } else {
            // Unrecognised condition — walk bodies under the current gate unchanged.
            walkNode(n.List,     gate, funcMap, boolSchema, conds, always)
            walkNode(n.ElseList, gate, funcMap, boolSchema, conds, always)
        }

    case *parse.ActionNode:
        walkNode(n.Pipe, gate, funcMap, boolSchema, conds, always)

    case *parse.PipeNode:
        for _, cmd := range n.Cmds {
            walkNode(cmd, gate, funcMap, boolSchema, conds, always)
        }

    case *parse.CommandNode:
        for _, arg := range n.Args {
            walkNode(arg, gate, funcMap, boolSchema, conds, always)
        }

    case *parse.FieldNode:
        if len(n.Ident) == 0 { return }
        key := n.Ident[0]
        if gate == nil {
            always[key] = true
        } else if !always[key] {
            if existing, exists := conds[key]; exists {
                // Already seen under a different condition — treat as always needed.
                if fmt.Sprint(existing) != fmt.Sprint(gate) {
                    always[key] = true
                    delete(conds, key)
                }
            } else {
                conds[key] = gate
            }
        }

    case *parse.RangeNode:
        walkNode(n.Pipe, gate, funcMap, boolSchema, conds, always)
        walkNode(n.List, gate, funcMap, boolSchema, conds, always)
        walkNode(n.ElseList, gate, funcMap, boolSchema, conds, always)

    case *parse.WithNode:
        walkNode(n.Pipe, gate, funcMap, boolSchema, conds, always)
        walkNode(n.List, gate, funcMap, boolSchema, conds, always)
        walkNode(n.ElseList, gate, funcMap, boolSchema, conds, always)
    }
}

// andGates combines an outer gate with an inner condition into a conjunction.
// If outer is nil, inner is returned as-is.
func andGates(outer Cond, inner Cond) Cond {
    if outer == nil { return inner }
    return condAnd{subs: []Cond{outer, inner}}
}
```

#### `parsePipeCond` — convert a pipe node to a `Cond`

```go
// parsePipeCond attempts to parse a template pipe node into a typed Cond.
// Returns (cond, true) on success, (nil, false) for any unrecognised form.
func parsePipeCond(pipe *parse.PipeNode) (Cond, bool) {
    if pipe == nil || len(pipe.Cmds) != 1 { return nil, false }
    return parseCmdCond(pipe.Cmds[0])
}

func parseCmdCond(cmd *parse.CommandNode) (Cond, bool) {
    args := cmd.Args
    if len(args) == 0 { return nil, false }

    // [[if .Var]] — single field reference
    if len(args) == 1 {
        if f, ok := args[0].(*parse.FieldNode); ok && len(f.Ident) == 1 {
            return condField{f.Ident[0]}, true
        }
        return nil, false
    }

    // Function form: first arg must be an identifier naming the function.
    fn, ok := args[0].(*parse.IdentifierNode)
    if !ok { return nil, false }

    switch fn.Ident {

    case "not": // [[if not …]]
        if len(args) != 2 { return nil, false }
        sub, ok := parseArgCond(args[1])
        if !ok { return nil, false }
        return condNot{sub}, true

    case "eq", "ne": // [[if eq .Var "literal"]]
        if len(args) != 3 { return nil, false }
        field, ok := args[1].(*parse.FieldNode)
        if !ok || len(field.Ident) != 1 { return nil, false }
        lit, ok := parseLiteral(args[2])
        if !ok { return nil, false }
        if fn.Ident == "eq" { return condEq{field.Ident[0], lit}, true }
        return condNe{field.Ident[0], lit}, true

    case "and", "or": // [[if and … …]] / [[if or … …]]
        if len(args) < 3 { return nil, false }
        subs := make([]Cond, 0, len(args)-1)
        for _, arg := range args[1:] {
            sub, ok := parseArgCond(arg)
            if !ok { return nil, false }
            subs = append(subs, sub)
        }
        if fn.Ident == "and" { return condAnd{subs}, true }
        return condOr{subs}, true
    }

    return nil, false
}

// parseArgCond handles an argument that is either a bare field or a parenthesised
// sub-expression (PipeNode), enabling recursion for nested conditions.
func parseArgCond(arg parse.Node) (Cond, bool) {
    switch n := arg.(type) {
    case *parse.FieldNode:
        if len(n.Ident) == 1 { return condField{n.Ident[0]}, true }
    case *parse.PipeNode:
        return parsePipeCond(n)
    }
    return nil, false
}

// parseLiteral extracts a Go value from a string, bool, or number node.
func parseLiteral(node parse.Node) (any, bool) {
    switch n := node.(type) {
    case *parse.StringNode: return n.Text, true
    case *parse.BoolNode:   return n.True, true
    case *parse.NumberNode:
        if n.IsInt   { return n.Int64, true }
        if n.IsFloat { return n.Float64, true }
    }
    return nil, false
}
```

---

## Changes to existing files

### `pkg/template/template.go`

Add `Conditionals` field and populate it in `Get`:

```go
type Template struct {
    Root         string
    Context      map[string]any
    ComputedDefs map[string]string
    Conditionals conditionals       // new — varName → Cond; absent = always prompt
    Metadata     *Metadata
    cfg          Config
    logger       *slog.Logger
    funcMap      texttemplate.FuncMap
    verbatim     *VerbatimRules
}

func Get(templateRoot string, cfg Config, logger *slog.Logger) (*Template, error) {
    funcMap := FuncMap(cfg, logger)

    userCtx, computedDefs, err := LoadUserContext(templateRoot, funcMap)
    if err != nil { return nil, err }

    conds, err := AnalyzeConditionals(templateRoot, userCtx, funcMap)
    if err != nil { return nil, err }

    verbatim, err := LoadVerbatim(templateRoot)
    if err != nil { return nil, err }

    meta, _ := loadMetadata(templateRoot)

    return &Template{
        Root:         templateRoot,
        Context:      userCtx,
        ComputedDefs: computedDefs,
        Conditionals: conds,
        Metadata:     meta,
        cfg:          cfg,
        logger:       logger,
        funcMap:      funcMap,
        verbatim:     verbatim,
    }, nil
}
```

### `pkg/cmd/template_use.go`

Replace the single-pass `promptContext` with a two-pass version:

```go
// promptContext prompts the user for schema variables not already in provided.
// Pass 1 prompts always-needed variables (those absent from conditionals).
// Pass 2 prompts conditional variables whose condition is now satisfied.
func promptContext(
    ctx    map[string]any,
    schema map[string]any,
    conds  conditionals,
    provided map[string]bool,
) error {
    keys := sortedKeys(schema)

    var alwaysKeys, condKeys []string
    for _, k := range keys {
        if _, conditional := conds[k]; conditional {
            condKeys = append(condKeys, k)
        } else {
            alwaysKeys = append(alwaysKeys, k)
        }
    }

    // Pass 1 — always-needed variables.
    if err := runPromptPass(ctx, schema, alwaysKeys, provided); err != nil {
        return err
    }

    // Pass 2 — conditional variables whose condition is now satisfied.
    var neededCondKeys []string
    for _, k := range condKeys {
        if conds[k].Eval(ctx) {
            neededCondKeys = append(neededCondKeys, k)
        }
    }
    return runPromptPass(ctx, schema, neededCondKeys, provided)
}

// runPromptPass builds a huh form for the given keys and runs it.
// Results are written back into ctx.
func runPromptPass(
    ctx      map[string]any,
    schema   map[string]any,
    keys     []string,
    provided map[string]bool,
) error {
    var fields []huh.Field
    stringResults := make(map[string]*string)
    boolResults   := make(map[string]*bool)

    for _, key := range keys {
        if provided[key] { continue }
        // … same field-building logic as the current promptContext …
    }

    if len(fields) == 0 { return nil }

    if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
        return err
    }
    for k, p := range stringResults { ctx[k] = *p }
    for k, p := range boolResults   { ctx[k] = *p }
    return nil
}
```

Update the call site in `executeTemplate`:

```go
if err := promptContext(ctx, tmpl.Context, tmpl.Conditionals, provided); err != nil {
    return err
}
```

---

## Context merge order (updated)

```
1. LoadUserContext(templateRoot)
        defaults from project.yaml, referenced defaults resolved
   ↓
2. AnalyzeConditionals(templateRoot, userCtx, funcMap)
        conditionals map built from template AST
   ↓
3. Merge --values file
   ↓
4. Merge --arg flags
   ↓
5. Prompt — Pass 1: always-needed variables
        (skipped for pre-provided keys AND when --use-defaults)
   ↓
6. Prompt — Pass 2: conditional variables whose Cond.Eval(ctx) == true
        (same skip rules)
   ↓
7. ApplyComputed(ctx, defs)
   ↓
8. Execute template + run hooks
```

---

## Tests

### `pkg/template/cond_test.go` (new)

| Test | Condition | ctx | Expected |
|------|-----------|-----|----------|
| `TestCondField_True`  | `condField{"A"}` | `A: true`  | `true`  |
| `TestCondField_False` | `condField{"A"}` | `A: false` | `false` |
| `TestCondNot`         | `condNot{condField{"A"}}` | `A: true` | `false` |
| `TestCondEq_Match`    | `condEq{"T","pg"}` | `T: "pg"` | `true` |
| `TestCondEq_NoMatch`  | `condEq{"T","pg"}` | `T: "mysql"` | `false` |
| `TestCondAnd_AllTrue` | `condAnd{A,B}` | both true | `true` |
| `TestCondAnd_OneFalse`| `condAnd{A,B}` | one false | `false` |
| `TestCondOr_OneTrue`  | `condOr{A,B}` | one true | `true` |
| `TestCondOr_AllFalse` | `condOr{A,B}` | both false | `false` |

### `pkg/template/analysis_test.go` (new)

| Test | Template content | Schema | Expected conditionals |
|------|-----------------|--------|-----------------------|
| `TestAnalysis_Unconditional` | `[[.Name]]` | `Name: ""` | empty — always prompt |
| `TestAnalysis_SimpleGate_False` | `[[if .UseDB]][[.DbName]][[end]]` | `UseDB: false, DbName: ""` | `DbName → condField{"UseDB"}` |
| `TestAnalysis_SimpleGate_True` | same | `UseDB: true` | same (gate is always detected regardless of schema default) |
| `TestAnalysis_ElseBranch` | `[[if .UseDB]]…[[else]][[.NoDbMsg]][[end]]` | both present | `NoDbMsg → condNot{condField{"UseDB"}}` |
| `TestAnalysis_Not` | `[[if not .UseDB]][[.Fallback]][[end]]` | present | `Fallback → condNot{condField{"UseDB"}}` |
| `TestAnalysis_Eq` | `[[if eq .DbType "pg"]][[.PgPort]][[end]]` | present | `PgPort → condEq{"DbType","pg"}` |
| `TestAnalysis_And` | `[[if and .UseDB .UseSSL]][[.Cert]][[end]]` | present | `Cert → condAnd{…}` |
| `TestAnalysis_Nested` | `[[if .UseDB]][[if eq .DbType "pg"]][[.PgCfg]][[end]][[end]]` | present | `PgCfg → condAnd{condField{"UseDB"}, condEq{"DbType","pg"}}` |
| `TestAnalysis_BothBranches` | same var inside and outside if | present | absent from map |
| `TestAnalysis_UnknownFn` | `[[if myFunc .X]][[.Y]][[end]]` | present | `Y` absent (falls back to always) |
| `TestAnalysis_MultiFile` | var unconditional in one file, conditional in another | present | absent from map |
| `TestAnalysis_Filename` | conditional filename `[[if .UseDB]]db.env[[end]]` | present | `UseDB` absent (accessed as gate) |

### `pkg/cmd/template_use_test.go` (additions)

| Test | Template | Schema / args | Expected |
|------|----------|---------------|----------|
| `TestTemplateUse_ConditionalSkipped` | `[[if .UseDB]]DB=[[.DbName]][[end]]` | `UseDB: false`, `--use-defaults` | output file has no DB line; no error |
| `TestTemplateUse_ConditionalIncluded` | same | `UseDB: true`, `--use-defaults` | output contains `DB=mydb` |
| `TestTemplateUse_ConditionalArgOverride` | same | `--arg UseDB=true --use-defaults` | output contains default DbName |

---

## Key notes

- `conditionals` is an unexported type alias — callers in `pkg/cmd` access it only through
  `Template.Conditionals` and the `Cond.Eval` method.
- `parsePipeCond` returns `(nil, false)` for anything it cannot classify. The walker then
  treats the bodies as unconditional — the safe, conservative fallback.
- Gate variables (those referenced in a condition expression) are walked at the *current*
  gate level, not the inner one. This ensures the gate variable itself is always classified
  as "always needed" rather than self-conditionally needed.
- `--use-defaults` bypasses both prompt passes entirely; conditional variables still retain
  their schema defaults in the context.
- `--values` / `--arg` overrides are applied before prompting. A conditional variable
  explicitly provided via `--arg` is marked as provided and skipped in both passes.
- Computed values are resolved after both prompt passes, so they can safely reference
  conditional variables regardless of whether they were prompted.

---

## Implementation deviations

### `Conditionals` is exported (not unexported as planned)

The plan described `conditionals` as an unexported type. In practice, `pkg/cmd/template_use.go`
passes it as a function parameter in `promptContext`, which requires the type to be visible
from outside `pkg/template`. The type was therefore exported as `Conditionals`.

### `analyseExpr` uses `texttemplate.New` instead of `parse.New`

The plan showed `parse.New("").Parse(src, "[[", "]]", ...)` for the low-level parser call.
This fails for templates containing Go's built-in functions (`not`, `eq`, `and`, `or`, etc.)
because the raw parser requires those names to be present in the provided funcMap.
`texttemplate.New("").Delims("[[","]]").Funcs(funcMap).Parse(src)` was used instead; it
merges the built-ins automatically before parsing, and the resulting `.Tree` is equivalent.
