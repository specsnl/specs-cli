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
- All tests pass (context parsing, ignore matching, execute pipeline).

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
    ├── context.go
    ├── ignore.go
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

The template FuncMap: Sprig + custom functions.

```go
package template

import (
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "os"
    "os/user"
    "strconv"
    "strings"
    "text/template"

    "github.com/docker/go-units"
    "github.com/go-sprout/sprout"
    "github.com/go-sprout/sprout/registry/conversion"
    "github.com/go-sprout/sprout/registry/crypto"
    "github.com/go-sprout/sprout/registry/encoding"
    "github.com/go-sprout/sprout/registry/maps"
    "github.com/go-sprout/sprout/registry/numeric"
    "github.com/go-sprout/sprout/registry/random"
    "github.com/go-sprout/sprout/registry/regexp"
    "github.com/go-sprout/sprout/registry/semver"
    "github.com/go-sprout/sprout/registry/slices"
    "github.com/go-sprout/sprout/registry/std"
    "github.com/go-sprout/sprout/registry/strings"
    "github.com/go-sprout/sprout/registry/time"
    "github.com/go-sprout/sprout/registry/uniqueid"
    "github.com/sethvargo/go-password/password"
)

// FuncMap returns all template functions: Sprout registries + custom specs functions.
// The env and filesystem registries are intentionally omitted — templates must not
// read host environment variables or access arbitrary paths.
func FuncMap() template.FuncMap {
    handler := sprout.New()
    handler.AddRegistries(
        std.NewRegistry(),
        uniqueid.NewRegistry(),
        semver.NewRegistry(),
        time.NewRegistry(),
        strings.NewRegistry(),
        random.NewRegistry(),
        encoding.NewRegistry(),
        conversion.NewRegistry(),
        numeric.NewRegistry(),
        crypto.NewRegistry(),
        regexp.NewRegistry(),
        slices.NewRegistry(),
        maps.NewRegistry(),
    )
    m := handler.Build()

    m["hostname"] = func() string {
        h, _ := os.Hostname()
        return h
    }
    m["username"] = func() string {
        u, _ := user.Current()
        if u != nil {
            return u.Username
        }
        return ""
    }
    m["toBinary"] = func(n int) string {
        return strconv.FormatInt(int64(n), 2)
    }
    m["formatFilesize"] = func(bytes float64) string {
        return units.HumanSize(bytes)
    }
    m["toTitle"] = strings.Title
    m["password"] = func(length, digits, symbols int, noUpper, allowRepeat bool) string {
        p, _ := password.Generate(length, digits, symbols, noUpper, allowRepeat)
        return p
    }
    m["randomBase64"] = func(length int) string {
        b := make([]byte, length)
        rand.Read(b)
        return base64.StdEncoding.EncodeToString(b)[:length]
    }

    return m
}
```

**Why use sprout registries:** Sprout is the active successor to sprig (~100 helpers across
string, math, date, crypto, encoding, semver registries). Opt-in registries mean only needed
function groups are included. `env` and `filesystem` registries are excluded so templates
cannot read host environment variables or access arbitrary paths — important for untrusted
template downloads.

---

### `pkg/template/context.go`

Loads the variable schema from `project.yaml` (or `project.json` as a fallback) and resolves
any referenced defaults.

#### Context schema

A context is a `map[string]interface{}` where values are typed as follows:

| YAML value | Go type after unmarshal | Prompt type |
|------------|------------------------|-------------|
| `name: my-project` | `string` | free-text input |
| `useSonar: false` | `bool` | yes/no confirm |
| `license: [MIT, GPL]` | `[]interface{}` | select (first item = default) |
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

// ParseContext loads project.yaml (or project.json) from templateRoot and
// returns the variable map with referenced defaults resolved.
func ParseContext(templateRoot string) (map[string]interface{}, error) {
    raw, err := loadContextFile(templateRoot)
    if err != nil {
        return nil, err
    }
    return resolveReferencedDefaults(raw)
}

// loadContextFile reads project.yaml; falls back to project.json.
func loadContextFile(templateRoot string) (map[string]interface{}, error) {
    yamlPath := filepath.Join(templateRoot, specs.ProjectYAMLFile)
    if data, err := os.ReadFile(yamlPath); err == nil {
        var ctx map[string]interface{}
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
    var ctx map[string]interface{}
    if err := json.Unmarshal(data, &ctx); err != nil {
        return nil, fmt.Errorf("parsing %s: %w", specs.ProjectJSONFile, err)
    }
    return ctx, nil
}
```

#### `resolveReferencedDefaults`

```go
func resolveReferencedDefaults(ctx map[string]interface{}) (map[string]interface{}, error) {
    // 1. Find all keys with [[ ]] in their string value.
    // 2. Extract .Key references from the template expressions.
    // 3. Topological sort.
    // 4. Render each in order, updating ctx as we go.
    // Returns an error if a cycle is detected or a referenced key does not exist.
}
```

**Cycle detection:** if the sorted order cannot be completed (a key depends on itself, or A
depends on B and B depends on A), return a clear error naming the involved keys.

**Scope:** only top-level string keys are checked. Nested objects and list values are never
referenced defaults.

---

### `pkg/template/ignore.go`

Loads `.specsignore` and matches file paths against its patterns.

```go
package template

import (
    "bufio"
    "os"
    "path/filepath"
    "strings"

    "github.com/danwakefield/fnmatch"
)

// IgnoreRules holds compiled glob patterns from .specsignore.
type IgnoreRules struct {
    patterns []string
}

// LoadIgnore reads .specsignore from templateRoot. Returns an empty IgnoreRules
// (no patterns) if the file does not exist.
func LoadIgnore(templateRoot string) (*IgnoreRules, error) {
    path := filepath.Join(templateRoot, specs.IgnoreFile)
    f, err := os.Open(path)
    if os.IsNotExist(err) {
        return &IgnoreRules{}, nil
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
    return &IgnoreRules{patterns: patterns}, scanner.Err()
}

// Matches reports whether path (relative to the template root) matches any ignore pattern.
// path uses forward slashes regardless of OS.
func (r *IgnoreRules) Matches(path string) bool {
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
    Root     string                 // path to the template root (contains project.yaml + template/)
    Context  map[string]interface{} // variable map with defaults resolved
    Metadata *Metadata              // nil if __metadata.json is absent
    funcMap  template.FuncMap
    ignore   *IgnoreRules
}

// Get loads a template from templateRoot. The root must contain either project.yaml or
// project.json, and a template/ subdirectory.
func Get(templateRoot string) (*Template, error) {
    ctx, err := ParseContext(templateRoot)
    if err != nil {
        return nil, err
    }

    ignore, err := LoadIgnore(templateRoot)
    if err != nil {
        return nil, err
    }

    meta, _ := loadMetadata(templateRoot) // missing metadata is not an error

    return &Template{
        Root:     templateRoot,
        Context:  ctx,
        Metadata: meta,
        funcMap:  FuncMap(),
        ignore:   ignore,
    }, nil
}
```

#### Execute

```go
// Execute renders the template/ subdirectory into targetDir.
// targetDir must already exist.
func (t *Template) Execute(targetDir string) error {
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
        destRel, err := t.renderName(rel)
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
        if t.ignore.Matches(relForward) || isBinary(srcPath) {
            return copyFile(srcPath, destPath)
        }
        return t.renderFile(srcPath, destPath)
    })
}
```

#### Helpers

```go
// renderName renders a file/directory path template using [[ ]] delimiters.
func (t *Template) renderName(name string) (string, error) {
    tmpl, err := template.New("").Delims("[[", "]]").Funcs(t.funcMap).Parse(name)
    if err != nil {
        return "", err
    }
    var buf strings.Builder
    if err := tmpl.Execute(&buf, t.Context); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// renderFile renders a text file's content using [[ ]] delimiters.
// If the rendered content is whitespace-only, the destination file is not created.
func (t *Template) renderFile(srcPath, destPath string) error {
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
    if err := tmpl.Execute(&buf, t.Context); err != nil {
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
    for _, seg := range strings.Split(path, string(filepath.Separator)) {
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
| Select default | `License: [MIT, GPL]` | `ctx["License"]` is `[]interface{}{"MIT","GPL"}` |
| project.json fallback | no project.yaml, has project.json | parses successfully |
| Referenced default | `Slug: "[[toKebabCase .Name]]"`, `Name: My App` | `ctx["Slug"] == "my-app"` |
| Cyclic reference | A depends on B, B depends on A | returns error |

### `pkg/template/ignore_test.go`

| Test | Pattern | Path | Matches |
|------|---------|------|---------|
| Exact filename | `composer.lock` | `composer.lock` | true |
| Wildcard | `*.min.js` | `dist/app.min.js` | true |
| Wildcard no match | `*.min.js` | `dist/app.js` | false |
| Glob double-star | `vendor/**` | `vendor/autoload.php` | true |
| Comment line ignored | `# composer.lock` | `composer.lock` | false |
| Missing file | *(no .specsignore)* | any | false (empty rules) |

### `pkg/template/template_test.go`

Each test case creates a minimal template directory structure in `t.TempDir()`, calls
`Get()` + `Execute()`, then asserts the target directory contents.

| Test | Setup | Expected |
|------|-------|----------|
| `TestExecute_StaticFile` | `template/hello.txt` with `Hello [[.Name]]` | target has `hello.txt` with `Hello World` |
| `TestExecute_ConditionalFilename_True` | `template/[[if .UseX]]feature.txt[[end]]`, `UseX: true` | `feature.txt` exists |
| `TestExecute_ConditionalFilename_False` | same, `UseX: false` | `feature.txt` absent |
| `TestExecute_ConditionalDir_False` | dir `[[if .UseX]]subdir[[end]]`, `UseX: false` | `subdir/` absent |
| `TestExecute_VerbatimCopy` | `template/composer.lock`, `.specsignore: composer.lock` | file copied byte-for-byte |
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
- **`hooks` key in `project.yaml`:** the context parser must strip the `hooks` key before
  returning the context — it is configuration for the hook runner (Phase 4), not a user-facing
  template variable.
