# Phase 3 — Template Engine

## Goal

Load a `project.yaml`, collect context, and render a `template/` directory to a target path
using `[[ ]]` delimiters. No interactive prompts yet — context comes from defaults only (the
prompt layer is Phase 6). The engine must be fully testable in isolation.

## Done criteria

- `template.Get(path)` returns a `*Template` loaded from `project.yaml` (fallback `project.json`).
- `template.Execute(targetDir)` renders all files to the target directory.
- Conditional filenames, verbatim copy, binary detection, and whitespace-only deletion all
  work correctly.
- Computed values (from the `computed:` section) are resolved after defaults and merged into
  the context before rendering.
- All tests pass (context parsing, verbatim matching, computed values, execute pipeline).

---

## Dependencies

```
go get gopkg.in/yaml.v3
go get github.com/go-sprout/sprout
go get github.com/sethvargo/go-password/password
go get github.com/docker/go-units
go get github.com/danwakefield/fnmatch
```

---

## File overview

```
pkg/
└── template/
    ├── metadata.go
    ├── functions.go
    ├── specsregistry.go
    ├── context.go
    ├── verbatim.go
    └── template.go
```

---

## Files

### `pkg/template/metadata.go`

The `__metadata.json` written by `template download` / `template save` and read back by
`template list`.

```go
package template

import (
    "encoding/json"
    "time"
)

// Metadata is stored in __metadata.json inside each registered template.
type Metadata struct {
    Tag        string   `json:"Tag"`
    Repository string   `json:"Repository"`
    Created    JSONTime `json:"Created"`
}

// JSONTime wraps time.Time with RFC1123Z serialisation and a human-readable display.
type JSONTime struct {
    time.Time
}

func (t JSONTime) MarshalJSON() ([]byte, error) {
    return json.Marshal(t.Time.Format(time.RFC1123Z))
}

func (t *JSONTime) UnmarshalJSON(data []byte) error {
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    parsed, err := time.Parse(time.RFC1123Z, s)
    if err != nil {
        return err
    }
    t.Time = parsed
    return nil
}

// String returns a human-readable relative time string ("3 days ago").
func (t JSONTime) String() string {
    d := time.Since(t.Time)
    switch {
    case d < time.Minute:
        return "just now"
    case d < time.Hour:
        return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
    case d < 24*time.Hour:
        return fmt.Sprintf("%d hours ago", int(d.Hours()))
    default:
        return fmt.Sprintf("%d days ago", int(d.Hours()/24))
    }
}
```

---

### `pkg/template/functions.go`

Builds the template FuncMap by wiring up all sprout registries and the custom `SpecsRegistry`.
In safe mode (`--safe-mode` flag), the `env` and `filesystem` registries are excluded so
templates cannot read host environment variables or access arbitrary paths — important for
untrusted template downloads.

```go
func FuncMap(cfg Config, logger *slog.Logger) texttemplate.FuncMap {
    registries := []sprout.Registry{
        sproutstd.NewRegistry(), sproutconversion.NewRegistry(), sproutnumeric.NewRegistry(),
        sproutreflect.NewRegistry(), sproutstrings.NewRegistry(), sproutencoding.NewRegistry(),
        sproutregexp.NewRegistry(), sproutslices.NewRegistry(), sproutmaps.NewRegistry(),
        sprouttime.NewRegistry(), sproutuniqueid.NewRegistry(), sproutcrypto.NewRegistry(),
        sproutchecksum.NewRegistry(), sproutnetwork.NewRegistry(), sproutsemver.NewRegistry(),
        sproutrandom.NewRegistry(),
        NewSpecsRegistry(), // specs-specific functions
    }

    if !cfg.SafeMode {
        registries = append(registries, sproutenv.NewRegistry(), sproutfilesystem.NewRegistry())
    }

    handler := sprout.New(sprout.WithLogger(logger))
    handler.AddRegistries(registries...)
    return handler.Build()
}
```

---

### `pkg/template/specsregistry.go`

A sprout `Registry` that provides specs-specific template functions. Registered via
`NewSpecsRegistry()` in `FuncMap`.

Functions provided:

| Function | Description |
|---|---|
| `hostname` | Returns the system hostname |
| `username` | Returns the current OS username |
| `toBinary` | Formats an integer as a binary string |
| `formatFilesize` | Human-readable file size (e.g. `"1.2 MB"`) via `docker/go-units` |
| `password` | Generates a secure random password via `sethvargo/go-password` |

---

### `pkg/template/context.go`

Loads the variable schema from `project.yaml` (or `project.json` as a fallback) and resolves
any referenced defaults.

#### Context schema

A context is a `map[string]any` where values are typed as follows:

| YAML value | Go type after unmarshal | Prompt type |
|------------|------------------------|-------------|
| `name: my-project` | `string` | free-text input |
| `useSonar: false` | `bool` | yes/no confirm |
| `license: [MIT, GPL]` | `[]any` | select (first item = default) |
| `ProjectSlug: "[[toKebabCase .Name]]"` | `string` containing `[[` | referenced default |

#### Referenced defaults

A string default that contains `[[` is a *referenced default* — it is itself a template
expression that depends on another context key. Before the user is prompted, these must be
resolved in dependency order so each key's pre-filled value is correct.

**Algorithm:**

1. Walk the flat context map. Identify keys whose string value contains `[[`.
2. For each such key, parse the template expression and extract `.KeyName` references.
3. Build a dependency graph: `ProjectSlug → {ProjectShortName}`.
4. Topological sort (Kahn's algorithm). Error on cycles.
5. Process keys in sorted order: render each referenced default using the current resolved
   context as data. The result replaces the original template expression string.

The context loading is split into two functions to support the computed values resolution
order (see [11-computed-values.md](../../11-computed-values.md)):

```go
package template

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "text/template"

    "gopkg.in/yaml.v3"
)

// LoadUserContext loads project.yaml (or project.json) from templateRoot.
// Strips the reserved top-level keys "computed" and "hooks" before returning.
// Resolves referenced defaults (string values containing "[[") in topological order.
// Returns the user input map and the raw computed definitions separately.
// funcMap is created once by Get() and passed in to avoid duplicate construction.
func LoadUserContext(templateRoot string, funcMap template.FuncMap) (userCtx map[string]any, computedDefs map[string]string, err error) {
    raw, err := loadContextFile(templateRoot)
    if err != nil {
        return nil, nil, err
    }
    computedDefs, err = extractComputed(raw) // strips "computed" key, returns its entries
    if err != nil {
        return nil, nil, err
    }
    delete(raw, "hooks") // hooks section is config, not a template variable
    userCtx, err = resolveReferencedDefaults(raw, funcMap)
    return userCtx, computedDefs, err
}

// ApplyComputed resolves computed definitions against the finalised context (post-prompt)
// and merges the results in. Called after prompting and --values/--arg overrides are complete.
// Returns a new map containing both user inputs and computed values.
func ApplyComputed(ctx map[string]any, defs map[string]string, funcMap template.FuncMap) (map[string]any, error) {
    // 1. Topological sort of defs by .Key dependencies.
    // 2. Execute each template in order against the growing context.
    // 3. Merge result into context before moving to the next computed key.
    // 4. Return error on cycle, unknown reference, or template execution failure.
}

// loadContextFile reads project.yaml; falls back to project.json.
func loadContextFile(templateRoot string) (map[string]any, error) {
    yamlPath := filepath.Join(templateRoot, specs.ProjectYAMLFile)
    if data, err := os.ReadFile(yamlPath); err == nil {
        var ctx map[string]any
        if err := yaml.Unmarshal(data, &ctx); err != nil {
            return nil, fmt.Errorf("parsing %s: %w", specs.ProjectYAMLFile, err)
        }
        return ctx, nil
    }

    jsonPath := filepath.Join(templateRoot, specs.ProjectJSONFile)
    data, err := os.ReadFile(jsonPath)
    if err != nil {
        return nil, fmt.Errorf("no project.yaml or project.json found in %s", templateRoot)
    }
    var ctx map[string]any
    if err := json.Unmarshal(data, &ctx); err != nil {
        return nil, fmt.Errorf("parsing %s: %w", specs.ProjectJSONFile, err)
    }
    return ctx, nil
}

// extractComputed removes the "computed" key from raw and returns its string entries.
// Returns an error if a computed key conflicts with a user input key.
func extractComputed(raw map[string]any) (map[string]string, error) {
    // ...
}
```

#### `resolveReferencedDefaults`

```go
func resolveReferencedDefaults(ctx map[string]any, funcMap template.FuncMap) (map[string]any, error) {
    // 1. Find all keys with [[ ]] in their string value.
    // 2. Extract .Key references from the template expressions.
    // 3. Topological sort (Kahn's algorithm).
    // 4. Render each in order, updating ctx as we go.
    // Returns an error if a cycle is detected or a referenced key does not exist.
}
```

**Cycle detection:** if the sorted order cannot be completed (a key depends on itself, or A
depends on B and B depends on A), return a clear error naming the involved keys.

**Scope:** only top-level string keys are checked. Nested objects and list values are never
referenced defaults.

**Note:** `resolveReferencedDefaults` and `ApplyComputed` share the same topological sort
logic — extract into a private `topoSort(deps map[string][]string) ([]string, error)` helper.

---

### `pkg/template/verbatim.go`

Loads `.specsverbatim` and matches file paths against its patterns. Matched files are copied
verbatim (no template rendering), identical to binary files.

```go
package template

import (
    "bufio"
    "os"
    "path/filepath"
    "strings"

    "github.com/danwakefield/fnmatch"
)

// VerbatimRules holds compiled glob patterns from .specsverbatim.
type VerbatimRules struct {
    patterns []string
}

// LoadVerbatim reads .specsverbatim from templateRoot. Returns an empty VerbatimRules
// (no patterns) if the file does not exist.
func LoadVerbatim(templateRoot string) (*VerbatimRules, error) {
    path := filepath.Join(templateRoot, specs.VerbatimFile)
    f, err := os.Open(path)
    if os.IsNotExist(err) {
        return &VerbatimRules{}, nil
    }
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var patterns []string
    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue // skip blanks and comments
        }
        patterns = append(patterns, line)
    }
    return &VerbatimRules{patterns: patterns}, scanner.Err()
}

// Matches reports whether path (relative to the template root) matches any verbatim pattern.
// path uses forward slashes regardless of OS.
func (r *VerbatimRules) Matches(path string) bool {
    for _, pattern := range r.patterns {
        if fnmatch.Match(pattern, path, 0) {
            return true
        }
    }
    return false
}
```

**Path normalisation:** always pass the path relative to the `template/` root, with forward
slashes. Example: `"vendor/autoload.php"` not `"template/vendor/autoload.php"`.

---

### `pkg/template/template.go`

The core engine. `Get` loads everything; `Execute` renders the template tree.

#### Struct and constructor

```go
package template

import (
    "io"
    "io/fs"
    "os"
    "path/filepath"
    "strings"
    "text/template"
    "unicode/utf8"
)

// ignoredFiles are always skipped — they are OS/editor metadata, not template content.
var ignoredFiles = map[string]bool{
    ".DS_Store": true,
    "Thumbs.db": true,
}

// Template holds everything needed to execute a template.
type Template struct {
    Root         string                 // path to the template root (contains project.yaml + template/)
    Context      map[string]any // user input map with referenced defaults resolved
    ComputedDefs map[string]string      // raw computed definitions; resolved by ApplyComputed post-prompt
    Metadata     *Metadata              // nil if __metadata.json is absent
    cfg          Config
    logger       *slog.Logger
    funcMap      template.FuncMap
    verbatim     *VerbatimRules
}

// Get loads a template from templateRoot. The root must contain either project.yaml or
// project.json, and a template/ subdirectory.
func Get(templateRoot string, cfg Config, logger *slog.Logger) (*Template, error) {
    funcMap := FuncMap(cfg, logger)

    userCtx, computedDefs, err := LoadUserContext(templateRoot, funcMap)
    if err != nil {
        return nil, err
    }

    verbatim, err := LoadVerbatim(templateRoot)
    if err != nil {
        return nil, err
    }

    meta, _ := loadMetadata(templateRoot) // missing metadata is not an error

    return &Template{
        Root:         templateRoot,
        Context:      userCtx,
        ComputedDefs: computedDefs,
        Metadata:     meta,
        cfg:          cfg,
        logger:       logger,
        funcMap:      funcMap,
        verbatim:     verbatim,
    }, nil
}
```

#### Execute

```go
// Execute renders the template/ subdirectory into targetDir.
// targetDir must already exist.
// If ComputedDefs is non-empty, ApplyComputed is called first to resolve and merge
// computed values into the context before the walk.
func (t *Template) Execute(targetDir string) error {
    ctx := t.Context
    if len(t.ComputedDefs) > 0 {
        var err error
        ctx, err = ApplyComputed(t.Context, t.ComputedDefs, t.funcMap)
        if err != nil {
            return err
        }
    }

    srcRoot := filepath.Join(t.Root, specs.TemplateDirFile)

    return filepath.WalkDir(srcRoot, func(srcPath string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }

        // Relative path from the template/ root (used for ignore matching and rendering)
        rel, _ := filepath.Rel(srcRoot, srcPath)
        if rel == "." {
            return nil // skip the root itself
        }

        // 1. Skip OS/editor junk files
        if ignoredFiles[d.Name()] {
            return nil
        }

        // 2. Render the relative path as a template to get the destination path.
        destRel, err := t.renderName(rel, ctx)
        if err != nil || strings.TrimSpace(destRel) == "" {
            // Parse/execution error or empty result: skip silently with a debug log.
            output.Debug("skipping %s: %v", rel, err)
            if d.IsDir() {
                return filepath.SkipDir
            }
            return nil
        }

        // 3. Check for empty path segments (conditional directory exclusion).
        if hasEmptySegment(destRel) {
            if d.IsDir() {
                return filepath.SkipDir
            }
            return nil
        }

        destPath := filepath.Join(targetDir, destRel)

        // 4. Directory: create it.
        if d.IsDir() {
            return os.MkdirAll(destPath, 0755)
        }

        // 5. File: determine copy strategy.
        relForward := filepath.ToSlash(rel)
        if t.verbatim.Matches(relForward) || isBinary(srcPath) {
            return copyFile(srcPath, destPath)
        }
        return t.renderFile(srcPath, destPath, ctx)
    })
}
```

#### Helpers

```go
// renderName renders a file/directory path template using [[ ]] delimiters.
// ctx is the fully resolved context (user inputs + computed values).
func (t *Template) renderName(name string, ctx map[string]any) (string, error) {
    tmpl, err := template.New("").Delims("[[", "]]").Funcs(t.funcMap).Parse(name)
    if err != nil {
        return "", err
    }
    var buf strings.Builder
    if err := tmpl.Execute(&buf, ctx); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// renderFile renders a text file's content using [[ ]] delimiters.
// ctx is the fully resolved context (user inputs + computed values).
// If the rendered content is whitespace-only, the destination file is not created.
func (t *Template) renderFile(srcPath, destPath string, ctx map[string]any) error {
    data, err := os.ReadFile(srcPath)
    if err != nil {
        return err
    }

    tmpl, err := template.New("").
        Delims("[[", "]]").
        Funcs(t.funcMap).
        Option("missingkey=error").
        Parse(string(data))
    if err != nil {
        // Malformed template: copy verbatim and log
        output.Debug("template parse error in %s, copying verbatim: %v", srcPath, err)
        return copyFile(srcPath, destPath)
    }

    var buf strings.Builder
    if err := tmpl.Execute(&buf, ctx); err != nil {
        output.Debug("template execute error in %s, copying verbatim: %v", srcPath, err)
        return copyFile(srcPath, destPath)
    }

    if strings.TrimSpace(buf.String()) == "" {
        return nil // whitespace-only: skip
    }

    return writeFile(destPath, []byte(buf.String()))
}

// isBinary returns true if the file contains a null byte or invalid UTF-8.
// Only the first 512 bytes are examined for performance.
func isBinary(path string) bool {
    f, err := os.Open(path)
    if err != nil {
        return false
    }
    defer f.Close()

    buf := make([]byte, 512)
    n, _ := f.Read(buf)
    buf = buf[:n]

    for _, b := range buf {
        if b == 0 {
            return true
        }
    }
    return !utf8.Valid(buf)
}

// hasEmptySegment returns true if any path segment (split by os.PathSeparator) is empty
// after trimming whitespace.
func hasEmptySegment(path string) bool {
    for seg := range strings.SplitSeq(path, string(filepath.Separator)) {
        if strings.TrimSpace(seg) == "" {
            return true
        }
    }
    return false
}

func copyFile(src, dst string) error {
    if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
        return err
    }
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()
    out, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer out.Close()
    _, err = io.Copy(out, in)
    return err
}

func writeFile(path string, data []byte) error {
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return err
    }
    return os.WriteFile(path, data, 0644)
}
```

---

## Tests

All tests use `t.TempDir()` for both source and target directories. No test touches the real
filesystem outside temp dirs.

### `pkg/template/context_test.go`

Table-driven; create a `project.yaml` in a temp dir for each case.

| Test | Input | Expected |
|------|-------|----------|
| String default | `Name: my-project` | `ctx["Name"] == "my-project"` |
| Bool default | `UseSonar: false` | `ctx["UseSonar"] == false` |
| Select default | `License: [MIT, GPL]` | `ctx["License"]` is `[]any{"MIT","GPL"}` |
| project.json fallback | no project.yaml, has project.json | parses successfully |
| Referenced default | `Slug: "[[toKebabCase .Name]]"`, `Name: My App` | `ctx["Slug"] == "my-app"` |
| Cyclic reference | A depends on B, B depends on A | returns error |
| Computed stripped | `computed:\n  Env: "prod"` | `computedDefs["Env"] == "prod"`, not in userCtx |
| Computed conflict | `Name: foo` + `computed:\n  Name: bar` | `LoadUserContext` returns error |
| Computed resolved | `Name: acme`, `computed:\n  Env: "[[toUpperCase .Name]]"` | `ApplyComputed` → `ctx["Env"] == "ACME"` |
| Computed chain | `computed:\n  A: "x"\n  B: "[[.A]]y"` | `ApplyComputed` → `ctx["B"] == "xy"` |
| Computed cycle | `computed:\n  A: "[[.B]]"\n  B: "[[.A]]"` | `ApplyComputed` returns error |

### `pkg/template/verbatim_test.go`

| Test | Pattern | Path | Matches |
|------|---------|------|---------|
| Exact filename | `composer.lock` | `composer.lock` | true |
| Wildcard | `*.min.js` | `dist/app.min.js` | true |
| Wildcard no match | `*.min.js` | `dist/app.js` | false |
| Glob double-star | `vendor/**` | `vendor/autoload.php` | true |
| Comment line ignored | `# composer.lock` | `composer.lock` | false |
| Missing file | *(no .specsverbatim)* | any | false (empty rules) |

### `pkg/template/template_test.go`

Each test case creates a minimal template directory structure in `t.TempDir()`, calls
`Get()` + `Execute()`, then asserts the target directory contents.

| Test | Setup | Expected |
|------|-------|----------|
| `TestExecute_StaticFile` | `template/hello.txt` with `Hello [[.Name]]` | target has `hello.txt` with `Hello World` |
| `TestExecute_ConditionalFilename_True` | `template/[[if .UseX]]feature.txt[[end]]`, `UseX: true` | `feature.txt` exists |
| `TestExecute_ConditionalFilename_False` | same, `UseX: false` | `feature.txt` absent |
| `TestExecute_ConditionalDir_False` | dir `[[if .UseX]]subdir[[end]]`, `UseX: false` | `subdir/` absent |
| `TestExecute_VerbatimCopy` | `template/composer.lock`, `.specsverbatim: composer.lock` | file copied byte-for-byte |
| `TestExecute_BinaryFile` | `template/image.png` with null bytes | file copied byte-for-byte |
| `TestExecute_WhitespaceOnly` | `template/empty.txt` with `[[if false]]x[[end]]` | `empty.txt` absent |
| `TestExecute_NestedConditionalDir` | dir `[[if .X]]a/b[[end]]`, file inside, `X: false` | neither dir nor file created |

---

## Key notes

- **`missingkey=error`** on content templates: any variable reference in a template file that
  is not in the context causes an error, preventing silent empty substitution. This matches
  v1 behaviour.
- **Filename templates do not use `missingkey=error`**: a parse or execute error on a filename
  is recoverable (skip + debug log), not fatal.
- **`text/template` vs `html/template`**: always use `text/template`. `html/template` escapes
  HTML entities, which would corrupt non-HTML files.
- **`project.json` fallback:** only the _presence_ of `project.yaml` is checked. If absent,
  fall back to JSON. The internal representation is identical either way.
- **YAML type coercion gotcha:** unquoted `8.4` in YAML is parsed as `float64`. Warn template
  authors in `specs template validate` output: quote strings that look like numbers.
- **Reserved keys stripped:** `LoadUserContext` removes both `hooks` and `computed` from the
  user input map before returning. `hooks` is consumed by the hook runner (Phase 4); `computed`
  is consumed by `ApplyComputed`. Neither should appear as prompted variables.
- **Computed values:** see [11-computed-values.md](../../11-computed-values.md) for the full
  design. `ApplyComputed` shares the same topological sort helper as `resolveReferencedDefaults`;
  extract it as a private `topoSort` function to avoid duplication.
