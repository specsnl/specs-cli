# Phase 7 — `specs template use`

## Goal

The main interactive command. Build a `huh` form from a `project.yaml` schema, merge
`--values` / `--arg` overrides, run pre/post-use hooks, and render the template into the
target directory.

## Done criteria

- `specs template use <tag> <target-dir>` prompts the user for each context key.
- `--use-defaults` skips all prompts and uses the schema defaults.
- `--values <file>` pre-fills answers from a JSON file; matching keys are not prompted.
- `--arg Key=Value` (repeatable) pre-fills individual keys; takes precedence over `--values`.
- `--no-hooks` skips pre/post-use hook execution.
- Computed values are resolved after all user inputs are finalised.
- Pre-use hook runs before rendering; post-use hook runs after copying to the target.
- All tests pass.

---

## Dependencies

```
go get charm.land/huh/v2
```

---

## File overview

```
pkg/
├── cmd/
│   └── template_use.go    (new)
└── util/
    └── values/
        └── values.go      (new: --values file loading and --arg parsing)
```

---

## Context merge order

```
1. LoadUserContext(templateRoot)
        defaults from project.yaml, referenced defaults resolved
   ↓
2. Merge --values file      (keys present override defaults)
   ↓
3. Merge --arg flags        (take precedence over --values)
   ↓
4. Prompt with huh form    (skipped for pre-provided keys AND when --use-defaults)
        user's answers update the context
   ↓
5. ApplyComputed(ctx, defs) (computed values always derived from final inputs)
   ↓
6. Execute template + run hooks
```

---

## Files

### `pkg/util/values/values.go`

Loads a JSON `--values` file and parses `--arg Key=Value` strings.

```go
package values

import (
    "encoding/json"
    "fmt"
    "os"
    "strings"
)

// LoadFile reads a JSON file and returns a flat map of key/value overrides.
// Only top-level string, bool, and number values are supported; nested objects are ignored.
func LoadFile(path string) (map[string]any, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading values file %q: %w", path, err)
    }
    var m map[string]any
    if err := json.Unmarshal(data, &m); err != nil {
        return nil, fmt.Errorf("parsing values file %q: %w", path, err)
    }
    return m, nil
}

// ParseArg parses a "Key=Value" string into its key and value parts.
// Returns an error if the string does not contain "=".
func ParseArg(arg string) (key, value string, err error) {
    parts := strings.SplitN(arg, "=", 2)
    if len(parts) != 2 {
        return "", "", fmt.Errorf("--arg %q must be in Key=Value form", arg)
    }
    return parts[0], parts[1], nil
}

// Merge applies overrides on top of base, returning a new map.
// Keys in overrides replace matching keys in base.
func Merge(base, overrides map[string]any) map[string]any {
    result := make(map[string]any, len(base))
    for k, v := range base {
        result[k] = v
    }
    for k, v := range overrides {
        result[k] = v
    }
    return result
}
```

---

### `pkg/cmd/template_use.go`

#### Form building

The huh form is built by iterating over the context map. Keys pre-filled by `--values` or
`--arg` are skipped. The schema value type determines the field type:

| Go type of default value | huh field |
|---|---|
| `string` | `huh.NewInput()` with `Value(&result)` and default pre-filled |
| `bool` | `huh.NewConfirm()` with `Value(&result)` |
| `[]any` | `huh.NewSelect[string]()` with first item as selected default |

```go
package cmd

import (
    "fmt"
    "os"
    "path/filepath"

    "charm.land/huh/v2"
    "github.com/spf13/cobra"

    "github.com/specsnl/specs-cli/pkg/hooks"
    "github.com/specsnl/specs-cli/pkg/specs"
    pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
    "github.com/specsnl/specs-cli/pkg/util/osutil"
    "github.com/specsnl/specs-cli/pkg/util/output"
    "github.com/specsnl/specs-cli/pkg/util/values"
    "github.com/specsnl/specs-cli/pkg/util/validate"
)

var (
    templateUseValuesFile string
    templateUseArgs       []string
    templateUseDefaults   bool
    templateUseNoHooks    bool
)

var templateUseCmd = &cobra.Command{
    Use:   "use <tag> <target-dir>",
    Short: "Execute a registered template",
    Args:  cobra.ExactArgs(2),
    RunE:  runTemplateUse,
}

func init() {
    templateUseCmd.Flags().StringVar(&templateUseValuesFile, "values", "", "JSON file of pre-filled values")
    templateUseCmd.Flags().StringArrayVar(&templateUseArgs, "arg", nil, "Key=Value pair (repeatable)")
    templateUseCmd.Flags().BoolVar(&templateUseDefaults, "use-defaults", false, "Skip prompts; use schema defaults")
    templateUseCmd.Flags().BoolVar(&templateUseNoHooks, "no-hooks", false, "Skip pre/post-use hooks")
    templateCmd.AddCommand(templateUseCmd)
}

func runTemplateUse(cmd *cobra.Command, args []string) error {
    tag, targetDir := args[0], args[1]

    if err := validate.Tag(tag); err != nil {
        return err
    }
    if !specs.IsRegistryInitialised() {
        return specs.ErrRegistryNotInitialised
    }

    templateRoot := specs.TemplatePath(tag)
    if _, err := os.Stat(templateRoot); os.IsNotExist(err) {
        return fmt.Errorf("%w: %s", specs.ErrTemplateNotFound, tag)
    }

    return executeTemplate(templateRoot, targetDir, executeOpts{
        valuesFile:  templateUseValuesFile,
        argPairs:    templateUseArgs,
        useDefaults: templateUseDefaults,
        noHooks:     templateUseNoHooks,
    })
}
```

#### `executeTemplate` — shared execution logic

Extracted into its own function so that `specs use` (Phase 8) can reuse it.

```go
type executeOpts struct {
    valuesFile  string
    argPairs    []string
    useDefaults bool
    noHooks     bool
}

func executeTemplate(templateRoot, targetDir string, opts executeOpts) error {
    // 1. Load user context + computed definitions.
    tmpl, err := pkgtemplate.Get(templateRoot)
    if err != nil {
        return err
    }
    ctx := tmpl.Context

    // 2. Load hooks (needed before prompting so pre-use can run).
    // Pass the raw project config to hooks.Load via re-reading project.yaml.
    rawConfig, err := loadRawConfig(templateRoot)
    if err != nil {
        return err
    }
    h, err := hooks.Load(templateRoot, rawConfig)
    if err != nil {
        return err
    }

    // 3. Merge --values file.
    if opts.valuesFile != "" {
        fileVals, err := values.LoadFile(opts.valuesFile)
        if err != nil {
            return err
        }
        ctx = values.Merge(ctx, fileVals)
    }

    // 4. Merge --arg flags.
    for _, pair := range opts.argPairs {
        k, v, err := values.ParseArg(pair)
        if err != nil {
            return err
        }
        ctx[k] = v
    }

    // 5. Prompt (unless --use-defaults).
    if !opts.useDefaults {
        if err := promptContext(ctx, tmpl.Context); err != nil {
            return err
        }
    }

    // 6. Resolve computed values.
    ctx, err = pkgtemplate.ApplyComputed(ctx, tmpl.ComputedDefs, tmpl.FuncMap())
    if err != nil {
        return err
    }

    // 7. Pre-use hook.
    if !opts.noHooks && h.HasPreUse() {
        output.Info("running pre-use hook…")
        if err := h.Run("pre-use", templateRoot, ctx, tmpl.FuncMap()); err != nil {
            return err
        }
    }

    // 8. Render into a temp directory, then copy to target.
    tmp, err := os.MkdirTemp("", "specs-use-*")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmp)

    // Inject the resolved context into the template before executing.
    tmpl.Context = ctx
    tmpl.ComputedDefs = nil // already applied
    if err := tmpl.Execute(tmp); err != nil {
        return err
    }

    if err := os.MkdirAll(targetDir, 0755); err != nil {
        return err
    }
    if err := osutil.CopyDir(tmp, targetDir); err != nil {
        return err
    }

    // 9. Post-use hook.
    if !opts.noHooks && h.HasPostUse() {
        output.Info("running post-use hook…")
        if err := h.Run("post-use", targetDir, ctx, tmpl.FuncMap()); err != nil {
            return err
        }
    }

    output.Info("done — files written to %s", targetDir)
    return nil
}
```

#### `promptContext` — huh form

Iterates over schema keys in a stable order and builds a huh form. Keys already in
`providedCtx` (pre-filled by `--values`/`--arg`) are skipped.

```go
// promptContext builds and runs a huh form for all keys in schema that are not
// already present in ctx. Results are written directly into ctx.
func promptContext(ctx map[string]any, schema map[string]any) error {
    var fields []huh.Field

    // Sort keys for deterministic prompt order.
    keys := sortedKeys(schema)

    for _, key := range keys {
        defaultVal := schema[key]

        switch v := defaultVal.(type) {
        case string:
            // Use the value already in ctx (may have been pre-filled by --values/--arg).
            current := ""
            if s, ok := ctx[key].(string); ok {
                current = s
            }
            // Skip if user already provided this key.
            if _, provided := ctx[key]; provided && ctx[key] != defaultVal {
                continue
            }
            fields = append(fields, huh.NewInput().
                Title(key).
                Value(&current).
                Description("default: "+v),
            )
            // Close over key/current in a separate goroutine-safe way.
            k, c := key, &current
            defer func() { ctx[k] = *c }()

        case bool:
            current := v
            if b, ok := ctx[key].(bool); ok {
                current = b
            }
            if _, provided := ctx[key]; provided && ctx[key] != defaultVal {
                continue
            }
            k, c := key, &current
            fields = append(fields, huh.NewConfirm().Title(key).Value(c))
            defer func() { ctx[k] = *c }()

        case []any:
            // Select — first item is the default.
            opts := toStringOptions(v)
            if len(opts) == 0 {
                continue
            }
            selected := opts[0]
            if s, ok := ctx[key].(string); ok {
                selected = s
            }
            if _, provided := ctx[key]; provided && ctx[key] != defaultVal {
                continue
            }
            k, s := key, &selected
            fields = append(fields, huh.NewSelect[string]().
                Title(key).
                Options(huh.NewOptions(opts...)...).
                Value(s),
            )
            defer func() { ctx[k] = *s }()
        }
    }

    if len(fields) == 0 {
        return nil
    }

    return huh.NewForm(huh.NewGroup(fields...)).Run()
}
```

> **Note on `defer` approach:** the `defer` trick above captures closures that flush
> field results into `ctx` after `form.Run()` returns. An alternative (and arguably
> cleaner) approach is to build a separate `results map[string]any` and write
> to ctx in a post-loop pass. Choose whichever reads more clearly during implementation.

---

### `loadRawConfig` helper

`hooks.Load` needs the raw parsed YAML map (including the `hooks` key) which
`LoadUserContext` strips. A thin re-read is the cleanest solution:

```go
// loadRawConfig reads project.yaml (or project.json) without stripping any keys.
// Used to pass the raw "hooks" value to hooks.Load.
func loadRawConfig(templateRoot string) (map[string]any, error) {
    // Attempt YAML first, fall back to JSON — identical to context.go logic.
    // Keep this function private to pkg/cmd; do not expose it via pkg/template.
    yamlPath := filepath.Join(templateRoot, specs.ProjectYAMLFile)
    if data, err := os.ReadFile(yamlPath); err == nil {
        var m map[string]any
        _ = yaml.Unmarshal(data, &m)
        return m, nil
    }
    jsonPath := filepath.Join(templateRoot, specs.ProjectJSONFile)
    data, err := os.ReadFile(jsonPath)
    if err != nil {
        return nil, err
    }
    var m map[string]any
    _ = json.Unmarshal(data, &m)
    return m, nil
}
```

---

### `Template.FuncMap()` — expose for callers

`executeTemplate` needs access to the FuncMap to pass to `hooks.Run` and `ApplyComputed`.
Add a public method to `pkg/template/Template`:

```go
// FuncMap returns the template's function map.
// Used by callers that need to pass the same FuncMap to hooks or ApplyComputed.
func (t *Template) FuncMap() texttemplate.FuncMap {
    return t.funcMap
}
```

---

## Tests

### `pkg/util/values/values_test.go`

| Test | Setup | Expected |
|---|---|---|
| `TestLoadFile_Valid` | JSON file `{"Name":"acme"}` | returns `map["Name":"acme"]` |
| `TestLoadFile_NotFound` | non-existent path | returns error |
| `TestLoadFile_InvalidJSON` | malformed JSON | returns error |
| `TestParseArg_Valid` | `"Name=acme"` | key=`Name`, value=`acme` |
| `TestParseArg_WithEquals` | `"Url=http://x.com"` | key=`Url`, value=`http://x.com` |
| `TestParseArg_NoEquals` | `"NoEquals"` | returns error |
| `TestMerge_OverridesBase` | base `{A:1}`, overrides `{A:2,B:3}` | `{A:2,B:3}` |
| `TestMerge_DoesNotMutateBase` | after merge, base unchanged | base still `{A:1}` |

### `pkg/cmd/template_use_test.go`

All tests build a real template directory in `t.TempDir()` and invoke the command.

| Test | Setup | Expected |
|---|---|---|
| `TestTemplateUse_UseDefaults` | `--use-defaults` flag | target dir has rendered files, no prompt |
| `TestTemplateUse_ArgOverride` | `--arg Name=test` | rendered file contains "test" |
| `TestTemplateUse_ValuesFile` | `--values` JSON file with `{"Name":"from-file"}` | rendered content matches |
| `TestTemplateUse_ArgBeatsValues` | `--values` sets Name=file, `--arg Name=arg` | arg value wins |
| `TestTemplateUse_NotFound` | unknown tag | returns `ErrTemplateNotFound` |
| `TestTemplateUse_NoHooks` | template with post-use hook, `--no-hooks` | hook script not executed |
| `TestTemplateUse_ComputedAvailable` | template with `computed:` section | computed key rendered in output |

---

## Key notes

- **`huh` standalone mode:** `form.Run()` blocks synchronously, exactly like a normal
  function call. No event loop, no goroutines needed. This is the primary reason huh was
  chosen.
- **Prompt order:** Go map iteration is random. Sort keys before building the form so
  the prompt order is deterministic and matches the order keys appear in `project.yaml`.
  Use `gopkg.in/yaml.v3`'s document order if needed, or simply sort alphabetically.
- **Pre-provided key detection:** a key is considered pre-provided if its value in `ctx`
  differs from the default in `schema`. This means a user who passes `--arg Name=default`
  still skips the prompt (the value matches the default but was explicitly supplied).
  Alternatively, track a separate `provided map[string]bool` set — this is cleaner and
  avoids the default-comparison ambiguity. Use this approach during implementation.
- **`tmpl.FuncMap()` exposure:** the `funcMap` field is unexported; the new public
  `FuncMap()` method is the minimal change needed to let callers access it without
  restructuring the package.
- **`--values` is JSON, not YAML.** The file format is JSON because the v1 plan specified
  it and it is simpler to parse for one-off CI automation. If a YAML `--values` file is
  desirable in future, `values.LoadFile` can be extended to detect by extension.
- **Temp directory cleanup:** `defer os.RemoveAll(tmp)` runs even if `Execute` or
  `CopyDir` fails — the partial output in `targetDir` may be incomplete, but the temp dir
  is always cleaned up.
- **`[]any` select values in `--arg`:** `--arg License=MIT` always produces a
  string. For a key whose schema type is `[]any` (a select), the string value is
  accepted as-is. No validation against the option list at this stage.
