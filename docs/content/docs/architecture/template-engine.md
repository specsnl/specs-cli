---
title: Template Engine
weight: 2
---

## Template Directory Convention

Every specs template is a directory with this structure:

```
<template-root>/
├── project.yml          # variable schema with defaults
├── .specsverbatim        # verbatim-copy glob patterns (opt-out from rendering)
├── __metadata.json       # written by specs (name, repo, created)
└── template/             # the files that get rendered
    ├── {{ .Name }}.go    # {{ }} template syntax in filenames
    ├── README.md         # {{ }} template syntax in file contents
    └── src/
        └── {{ .Package }}/
            └── main.go
```

Only the `template/` subdirectory is ever rendered and written to the target.

---

## `project.yml` — Context Schema

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
ProjectSlug: "{{ .ProjectShortName | toKebabCase }}"

# Computed — never prompted, always derived from final inputs
computed:
  DbName: "{{ .ProjectShortName | toSnakeCase }}_production"
  Year:   "{{ now | date \"2006\" }}"
```

| Value type | Prompt behaviour |
|------------|-----------------|
| `string`   | Free-text input with default shown |
| `bool`     | Yes/No confirm prompt |
| `[]string` | Select list; first item is default |
| `string` containing `{{` | Referenced default — pre-fill computed, user can override |
| `computed:` section | Never prompted — derived after all user inputs are finalised |

---

## Configurable Delimiters

Specs uses `{{ }}` by default — standard Go `text/template` syntax. To avoid conflicts with tools that also use `{{ }}` (e.g. GitHub Actions, Helm), you can override the delimiters per template using the reserved `__delimiters` key in `project.yml`:

```yaml
__delimiters:
  left: "[["
  right: "]]"
```

Both `left` and `right` must be non-empty strings. The `__delimiters` key is reserved and never exposed as a template variable.

### Reserved `__` namespace

The entire `__` prefix is reserved for specs configuration keys (such as `__delimiters`), so
that future configuration can be added without clashing with existing template variables.
A template may **not** define a user variable or a computed value whose name starts with `__`
unless it is a recognised configuration key. Any other `__`-prefixed name is rejected with
`ErrReservedVariableName` (`error_kind: reserved_variable_name`).

The check runs whenever a project file is loaded — during `template download`, `template use`
(and `specs use`), and `template validate` — so a template that misuses the namespace is
rejected at download time rather than only failing later at execution. Runtime overrides are
held to the same rule: a reserved name supplied via `--arg __foo=…` or a `--values` file is
also rejected.

```yaml
Name: my-project
__future: nope   # error: variable names starting with "__" are reserved
computed:
  __derived: "{{ .Name }}"   # error: computed names are reserved too
```

With `[[ ]]` delimiters configured, `{{ }}` in your files passes through unchanged:

```yaml
# With __delimiters: left: "[[", right: "]]"

# ${{ }} passes through unchanged — no escaping needed
group: ${{ github.workflow }}-${{ github.ref }}
SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}

# Template expressions use [[ ]]
MARIADB_DATABASE: "[[ .ProjectShortName | toSnakeCase ]]_test"
```

---

## Version Gate (`__specs__version`)

Before loading a template's context, `template.Get()` runs a pre-flight check against the
reserved `__specs__version` key in `project.yml`. It lets a template author declare which
`specs` CLI versions the template supports:

```yaml
__specs__version: ^0.1.0
```

The value is any valid [Masterminds/semver](https://github.com/Masterminds/semver) constraint
string (`0.1.0`, `^0.1.0`, `~0.1.0`, `>= 0.1.0, < 2.0`, …), so both lower and upper bounds can
be expressed. `template.ExtractSpecsVersion` parses it with `semver.NewConstraint`; the private
`checkSpecsVersion` helper then evaluates it against the running CLI version (injected via
`Config.Version` from the cmd layer, so `pkg/template` never imports `pkg/cmd`):

| Condition | Outcome |
|-----------|---------|
| Key absent | No check — proceed |
| Present but not a string, or not a parseable constraint | `ErrInvalidSpecsVersion` — refuse to load |
| `Config.Version` is empty, `dev`, or otherwise unparseable | Check skipped with a `slog.Debug` line — never lock out source builds |
| Parsed CLI version does not satisfy the constraint | `ErrSpecsVersionUnsatisfied` (wrapped with `%w`) |
| Parsed CLI version satisfies the constraint | Proceed |

The gate fires for both `specs use` and `specs template use` (which share `executeTemplate`).
`specs template save` never calls `Get()`, so a newer template can still be saved on an older
CLI. The reserved key is consumed during load and never exposed as a template variable.

---

## Always-ignored files

The following files are silently skipped during rendering, regardless of any other
configuration. They are OS/editor metadata that should never appear in scaffolded output:

| Filename | Origin |
|----------|--------|
| `.DS_Store` | macOS Finder |
| `Thumbs.db` | Windows Explorer |

These files are skipped at the walk stage — they are never copied, rendered, or passed to
`.specsverbatim`. This list is fixed; use `.specsignore` if per-template suppression is
ever added in the future.

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
└── {{ if .UseSonarQube }}sonar-project.properties{{ end }}
└── {{ if .UseSonarQube }}docs/images{{ end }}/
    └── badge.png
```

| `UseSonarQube` | Rendered path | Result |
|---|---|---|
| `true` | `sonar-project.properties` | created |
| `false` | *(empty)* | skipped |
| `false` (badge.png) | `/badge.png` | skipped — empty segment |

---

## Render Pipeline

```mermaid
flowchart TD
    A["Execute(targetDir)"] --> B["ApplyComputed if ComputedDefs non-empty\n(computed values resolved before walk)"]
    B --> C["filepath.WalkDir(template/)"]
    C --> D{ignoredFile?}
    D -->|yes| Skip1[skip]
    D -->|no| E[render path as template]
    E --> F{render error or result empty?}
    F -->|yes| Skip2[skip]
    F -->|no| G{any path segment empty?}
    G -->|yes| Skip3[skip dir tree]
    G -->|no| H{is directory?}
    H -->|yes| Mkdir[os.MkdirAll]
    H -->|no| I{matches .specsverbatim?}
    I -->|yes| Copy1[copy verbatim]
    I -->|no| J{isBinary?}
    J -->|yes| Copy2[copy verbatim]
    J -->|no| K[render content as template]
    K --> KE{parse or execution error?}
    KE -->|"yes — fail-fast (default)"| Abort["return error, no file written"]
    KE -->|"yes — --continue-on-error"| Warn["append RenderWarning\n(path + first 80 chars preview)\ncopy verbatim"]
    KE -->|no| L{whitespace-only result?}
    L -->|yes| Skip4[skip — do not create file]
    L -->|no| Write[write to dest]
```

### Render error modes

| Mode | Behaviour | How to enable |
|------|-----------|---------------|
| **Fail-fast** (default) | Parse or execution errors abort `Execute` immediately; no destination file is written for the affected path. The tmp dir is cleaned up by the deferred `os.RemoveAll` in `template use`. | Default — no flag needed |
| **Continue-on-error** | Parse or execution errors are recorded as `RenderWarning` entries and the file is copied verbatim. Restores the pre-v0.x behaviour. Use only when `.specsverbatim` is not an option. | `--continue-on-error` flag |

If any `RenderWarning` entries are present after `Execute` returns (only possible with `--continue-on-error`):

- **`template use`** reports each affected file via `Output.Warn`, including the destination
  path and the first 80 characters of the unrendered source so the user can grep for
  straggling `{{ }}` literals. It also suggests running `specs template validate <name>`.
- **`template validate`** reports each affected file, includes them in the exit code
  (`ValidateRender = 4`), and always runs at debug log level so no diagnostic output
  is suppressed.

### Binary Detection

The engine inspects the first 512 bytes of every file using a two-stage check. Binary files
are copied byte-for-byte; no template rendering is attempted.

**Detection order:**

1. **`http.DetectContentType`** — identifies known binary formats by magic bytes (JPEG, PNG,
   PDF, ZIP, gzip, and others). If the detected content type is not a `text/*` type, the
   file is treated as binary.
2. **Null-byte / invalid-UTF-8 fallback** — if `DetectContentType` returns `text/plain`,
   the file is still treated as binary when the first 512 bytes contain a null byte or are
   not valid UTF-8. This catches edge cases such as UTF-16 LE files (which have many null
   bytes) or binary payloads whose prefix happens to look like text.

**Limitations and `.specsverbatim`:**
Binary detection is best-effort. Any file that must be copied verbatim should be listed
explicitly in `.specsverbatim` rather than relying on auto-detection:

```
# .specsverbatim — recommended patterns for common binary assets
*.png
*.jpg
*.ico
*.woff
*.woff2
*.ttf
*.pdf
*.gz
*.tar
*.zip
```

### File Permissions

Source file permission bits (mode) are always preserved in the destination, regardless of
the copy strategy:

- **Binary / verbatim copy** (`copyFile`): the source `os.Stat` mode is applied to the
  destination via `os.OpenFile(..., info.Mode())` followed by `os.Chmod`.
- **Rendered text** (`writeFile`): `renderFile` stats the source before rendering and
  passes `info.Mode()` to `writeFile`, which applies it with `os.WriteFile` + `os.Chmod`.

The same preservation applies when a template is saved to the local registry
(`specs template save` / `specs use <local-path>`): `osutil.CopyDir` uses the same
`os.Stat` + `os.Chmod` pattern, so a script that is `chmod +x` in the source stays
executable in the registry and in every scaffolded output.

---

## Template Functions

All of Go's standard `text/template` built-ins are available, plus:

### Custom Functions (`internal/template/specsregistry.go`)

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
are guarded behind conditions (see `internal/template/analysis.go`). Prompting is iterative:

1. **Pass 1** — unconditional variables (always needed, regardless of any condition)
2. **Pass 2+** — each round finds conditional variables whose guard variables are all resolved,
   evaluates the condition against the current context, and prompts those that are needed.
   This repeats until no more conditional variables can be resolved.

Variables that appear nowhere in the template files or computed expressions are skipped
entirely — they are never prompted regardless of their presence in `project.yml`.

The condition types recognised by the AST analyser are:

| Template expression | Condition type |
|---|---|
| `{{ if .Var }}` | `condField` — truthy check |
| `{{ if not .Var }}` | `condNot` — negation |
| `{{ if eq .Var "value" }}` | `condEq` — equality |
| `{{ if ne .Var "value" }}` | `condNe` — inequality |
| `{{ if and .A .B }}` | `condAnd` — conjunction |
| `{{ if or .A .B }}` | `condOr` — disjunction |

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

**Form A — inline in `project.yml`:**
```yaml
hooks:
  pre-use:
    - echo "Scaffolding {{ .ProjectName }}..."
  post-use:
    - composer install
    - npm install
    - |
      git init
      git add -A
      git commit -m "Initial commit: {{ .ProjectName }}"
```

**Form B — script files:**
```
template-root/
├── project.yml
├── hooks/
│   ├── pre-use.sh
│   └── post-use.sh
└── template/
```

Context values are injected as `SPECS_`-prefixed uppercase env vars:
`ProjectName` → `SPECS_PROJECTNAME`.
The prefix can be disabled with the root `--no-env-prefix` flag.

### Trust model

Hooks run arbitrary shell commands on the host. Before running any template with hooks:

| Scenario | Behaviour |
|---|---|
| `--safe-mode` (no `--allow-hooks`) | Hooks are **skipped entirely** — no bash invocation |
| `--safe-mode --allow-hooks` | Function-level restrictions apply; hooks **run** (remote confirmation still required) |
| `specs use <remote>` with hooks | Rendered hook commands are **printed** and **interactive confirmation** is required |
| `specs use <remote> --yes` | Confirmation prompt is skipped; hooks run (CI use) |
| `specs use <remote> --no-hooks` | Hooks are **skipped** |
| Local template or registry template | No confirmation prompt; hooks run as normal |

**`--safe-mode` implies `--no-hooks`** in the command layer. Pass `--allow-hooks` alongside `--safe-mode` to disable only the env/filesystem template functions while still allowing hooks to execute.

When running a remote template interactively (`specs use github:user/repo ./out`), specs prints all pre-use and post-use hook commands (rendered against the resolved context) and asks for confirmation before executing any of them. Passing `--yes` suppresses this prompt for scripted or CI use.

If `bash` is not on `PATH`, hook execution returns an actionable error identifying the missing shell rather than a confusing process-not-found failure.

---

## Metadata (`template.Metadata`)

```go
type Metadata struct {
    Name       string   `json:"Name"`
    Repository string   `json:"Repository"`
    Branch     string   `json:"Branch,omitempty"`
    Created    JSONTime `json:"Created"`            // set on first install; preserved across upgrades
    Commit     string   `json:"Commit,omitempty"`   // full SHA-1 of HEAD at download/upgrade
    Version    string   `json:"Version,omitempty"`  // git-describe-style version string
}
```

`Created` records when the template was first added to the registry (via
`template download` or `template save`) and is intentionally preserved across
`template upgrade` so the `list` command's `Created` column reflects the
original install time, not the most recent upgrade. `Commit` and `Version` are
the only fields that change on upgrade.

`JSONTime` wraps `time.Time` with RFC1123Z serialisation and a human-readable `"X time ago"`
display format for the `list` command.

---

## Validation (`specs template validate`)

`specs template validate` always runs at debug log level so no diagnostic output is suppressed.
It calls `Execute` on a temporary directory first to catch render errors, then calls
`Template.Validate()` for static analysis.

Three categories of issues are reported:

| Issue kind | Source | Meaning |
|---|---|---|
| `render_error` | `Execute()` | A file could not be rendered (parse or execution error) and was copied verbatim. |
| `unknown_variable` | `Validate()` | A name used in a template file or path is **not defined** in `project.yaml`. |
| `unused_variable` | `Validate()` | A variable **defined** in the user input section is never referenced anywhere. |
| `unused_computed` | `Validate()` | A computed value **defined** under `computed:` is never referenced anywhere. |

Exit codes are a bitmask — multiple conditions combine additively:

| Bit | Constant | Value | Condition |
|-----|----------|-------|-----------|
| 2 | `ValidateRender` | 4 | Any `render_error` — file copied verbatim due to parse or execution error |
| 1 | `ValidateUnknown` | 2 | Any `unknown_variable` |
| 0 | `ValidateUnused` | 1 | Any `unused_*` (only with `--strict`) |

`specs template validate` exits 0 only when there are no render errors and no unknown variables.

---

## Template Status Tracking

`__status.json` caches the result of a remote HEAD check per template. The `list` command
refreshes stale entries (older than 24 hours) concurrently, bounded to at most 8 parallel
checks via `errgroup.SetLimit(8)`. Each individual check has a 10-second per-remote timeout;
the entire refresh phase has a 30-second top-level timeout. Both timeouts use `context.Context`
propagated from the command, so Ctrl-C cancels in-flight checks immediately.
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
