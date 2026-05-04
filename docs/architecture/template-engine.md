# Specs CLI — Template Engine

## Template Directory Convention

Every specs template is a directory with this structure:

```
<template-root>/
├── project.yaml          # variable schema with defaults
├── .specsverbatim        # verbatim-copy glob patterns (opt-out from rendering)
├── __metadata.json       # written by specs (name, repo, created)
└── template/             # the files that get rendered
    ├── [[ .Name ]].go    # [[ ]] template syntax in filenames
    ├── README.md         # [[ ]] template syntax in file contents
    └── src/
        └── [[ .Package ]]/
            └── main.go
```

Only the `template/` subdirectory is ever rendered and written to the target.

---

## `project.yaml` — Context Schema

Defines the variables that specs collects from the user (or uses as defaults).

```yaml
ProjectName: My Acme Project
ProjectShortName: acme-12

# Select — first value is the default
PhpVersion:
  - "8.5"
  - "8.4"
  - "8.3"

# Bool — false = no, true = yes
UseSonarQube: false

# Referenced default — shown as pre-fill, user can override
ProjectSlug: "[[ .ProjectShortName | toKebabCase ]]"

# Computed — never prompted, always derived from final inputs
computed:
  DbName: "[[ .ProjectShortName | toSnakeCase ]]_production"
  Year:   "[[ now | date \"2006\" ]]"
```

| Value type | Prompt behaviour |
|------------|-----------------|
| `string`   | Free-text input with default shown |
| `bool`     | Yes/No confirm prompt |
| `[]string` | Select list; first item is default |
| `string` containing `[[` | Referenced default — pre-fill computed, user can override |
| `computed:` section | Never prompted — derived after all user inputs are finalised |

---

## Delimiter Change: `{{ }}` → `[[ ]]`

Specs uses `[[ ]]` instead of `{{ }}` to avoid conflicts with other tools (e.g. GitHub Actions, Helm).

```go
template.New(name).Delims("[[", "]]").Funcs(funcMap).Parse(content)
```

This means GitHub Actions workflow files in templates work without escaping:

```yaml
# Works naturally with [[ ]] delimiters
group: ${{ github.workflow }}-${{ github.ref }}
SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}

# Template expressions use [[ ]]
MARIADB_DATABASE: "[[ .ProjectShortName | toSnakeCase ]]_test"
```

---

## `.specsverbatim` — Verbatim Copy

A `.specsverbatim` file at the template root lists glob patterns for files that should be
copied byte-for-byte without any template rendering:

```
# .specsverbatim
composer.lock
package-lock.json
*.min.js
vendor/**
```

---

## Conditional Files and Directories

Use the filename itself as a template expression. After rendering:

- Empty or whitespace result → skip the file/directory
- Any **path segment** empty → skip the file (enables conditional directory trees)

```
template/
└── [[ if .UseSonarQube ]]sonar-project.properties[[ end ]]
└── [[ if .UseSonarQube ]]docs/images[[ end ]]/
    └── badge.png
```

| `UseSonarQube` | Rendered path | Result |
|---|---|---|
| `true` | `sonar-project.properties` | created |
| `false` | `` (empty) | skipped |
| `false` (badge.png) | `/badge.png` | skipped — empty segment |

---

## Render Pipeline

```mermaid
flowchart TD
    A["Execute(targetDir)"] --> B["ApplyComputed if ComputedDefs non-empty\n(computed values resolved before walk)"]
    B --> C["filepath.WalkDir(template/)"]
    D --> E{ignoredFile?}
    E -->|yes| Skip1[skip]
    E -->|no| F[render path as template with \[\[ \]\] engine]
    F --> G{render error or result empty?}
    G -->|yes| Skip2[skip]
    G -->|no| H{any path segment empty?}
    H -->|yes| Skip3[skip dir tree]
    H -->|no| I{is directory?}
    I -->|yes| Mkdir[os.MkdirAll]
    I -->|no| J{matches .specsverbatim?}
    J -->|yes| Copy1[copy verbatim]
    J -->|no| K{isBinary?}
    K -->|yes| Copy2[copy verbatim]
    K -->|no| L[render content with \[\[ \]\] engine]
    L --> M{whitespace-only result?}
    M -->|yes| Skip4[skip — do not create file]
    M -->|no| Write[write to dest]
```

### Binary Detection

A file is treated as binary if the first 512 bytes contain a null byte or are not valid UTF-8.
Binary files are copied byte-for-byte; no template rendering is attempted.

---

## Template Functions

All of Go's standard `text/template` built-ins are available, plus:

### Custom Functions (`pkg/template/specsregistry.go`)

| Function | Signature | Description |
|----------|-----------|-------------|
| `hostname` | `() string` | System hostname |
| `username` | `() string` | Current OS username |
| `toBinary` | `(n int) string` | Format integer as binary string |
| `formatFilesize` | `(bytes float64) string` | Human-readable size (KB/MB/GB…) |
| `password` | `(length, digits, symbols int, noUpper, allowRepeat bool) string` | Secure random password |

### Sprout Functions

All functions from [`go-sprout/sprout`](https://github.com/go-sprout/sprout) are available —
~100 helpers for string manipulation, math, date/time, encoding, and more.

Key renamed functions vs the old sprig library:

| Old (sprig) | New (sprout) |
|---|---|
| `kebabcase` | `toKebabCase` |
| `snakecase` | `toSnakeCase` |
| `camelcase` | `toPascalCase` |
| `upper` | `toUpper` |
| `lower` | `toLower` |
| `title` | `toTitleCase` |
| `b64enc` / `b64dec` | `base64Encode` / `base64Decode` |

### Template Options

```go
tmpl.Option("missingkey=error")
```

Any variable referenced in a template that has no value in the context causes an error,
preventing silent empty substitutions.

### Safe Mode

When `--safe-mode` is set (or for untrusted template sources), the `env` and `filesystem`
Sprout registries are disabled — templates cannot read host environment variables or access
the filesystem beyond their own template directory.

---

## Iterative Conditional Prompting

Before prompting, specs analyses the template file tree's AST to determine which variables
are guarded behind conditions (see `pkg/template/analysis.go`). Prompting is iterative:

1. **Pass 1** — unconditional variables (always needed, regardless of any condition)
2. **Pass 2+** — each round finds conditional variables whose guard variables are all resolved,
   evaluates the condition against the current context, and prompts those that are needed.
   This repeats until no more conditional variables can be resolved.

Variables that appear nowhere in the template files or computed expressions are skipped
entirely — they are never prompted regardless of their presence in `project.yaml`.

The condition types recognised by the AST analyser are:

| Template expression | Condition type |
|---|---|
| `[[ if .Var ]]` | `condField` — truthy check |
| `[[ if not .Var ]]` | `condNot` — negation |
| `[[ if eq .Var "value" ]]` | `condEq` — equality |
| `[[ if ne .Var "value" ]]` | `condNe` — inequality |
| `[[ if and .A .B ]]` | `condAnd` — conjunction |
| `[[ if or .A .B ]]` | `condOr` — disjunction |

Unrecognised condition forms fall back to treating the variable as always-needed
(conservative: over-prompt rather than under-prompt).

---

## Hooks

Hooks run shell commands before and after `specs template use`. Two trigger points:

| Hook | Working directory | Runs |
|------|------------------|-------|
| `pre-use` | template source directory | Before any files are rendered. Non-zero exit aborts. |
| `post-use` | target (output) directory | After all files are written. Receives resolved context as `SPECS_`-prefixed env vars. |

Two mutually exclusive definition forms:

**Form A — inline in `project.yaml`:**
```yaml
hooks:
  pre-use:
    - echo "Scaffolding [[ .ProjectName ]]..."
  post-use:
    - composer install
    - npm install
    - |
      git init
      git add -A
      git commit -m "Initial commit: [[ .ProjectName ]]"
```

**Form B — script files:**
```
template-root/
├── project.yaml
├── hooks/
│   ├── pre-use.sh
│   └── post-use.sh
└── template/
```

Context values are injected as `SPECS_`-prefixed uppercase env vars:
`ProjectName` → `SPECS_PROJECTNAME`.
The prefix can be disabled with the root `--no-env-prefix` flag.

---

## Metadata (`template.Metadata`)

```go
type Metadata struct {
    Name       string   `json:"Name"`
    Repository string   `json:"Repository"`
    Branch     string   `json:"Branch,omitempty"`
    Created    JSONTime `json:"Created"`
    Commit     string   `json:"Commit,omitempty"`   // full SHA-1 of HEAD at download/upgrade
    Version    string   `json:"Version,omitempty"`  // git-describe-style version string
}
```

`JSONTime` wraps `time.Time` with RFC1123Z serialisation and a human-readable `"X time ago"`
display format for the `list` command.

---

## Validation (`specs template validate`)

`Template.Validate()` inspects the template for two categories of issues:

| Issue kind | Meaning |
|---|---|
| `unknown_variable` | A name used in a template file or path is **not defined** in `project.yaml` (neither variable nor computed). Reports the file path where the reference was found. |
| `unused_variable` | A variable **defined** in the user input section is never referenced in any template file, path expression, or computed expression. |
| `unused_computed` | A computed value **defined** under `computed:` is never referenced anywhere. |

`unknown_variable` issues are errors — the template will fail to render.
`unused_*` issues are warnings — they indicate dead schema entries.

`specs template validate` exits non-zero if any `unknown_variable` issues are found.

---

## Template Status Tracking

`__status.json` caches the result of a remote HEAD check per template. The `list` command
refreshes stale entries (older than 24 hours) concurrently using `sync.WaitGroup`.
The `update` command forces an immediate refresh for one or all templates.

```go
type TemplateStatus struct {
    CheckedAt     JSONTime              // time of last remote check
    IsUpToDate    bool                  // true when local HEAD matches remote
    LatestVersion string                // set when a newer semver tag is available
    ErrorKind     pkggit.CheckErrorKind // "network", "auth", "not-found", "unknown", or ""
}
```

`specs template list` displays a `Status` column with labels: `up-to-date`,
`update: <version>`, `update available`, `unknown (offline?)`, `auth error`, `not found`.
