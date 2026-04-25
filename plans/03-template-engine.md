# Boilr — Template Engine

## Template Directory Convention

Every boilr template is a directory with this structure:

```
<template-root>/
├── project.json          # variable schema with defaults
├── __metadata.json       # written by boilr (name, repo, created)
└── template/             # the files that get rendered
    ├── {{.Name}}.go      # Go template syntax in filenames
    ├── README.md         # Go template syntax in file contents
    └── src/
        └── {{.Package}}/
            └── main.go
```

Only the `template/` subdirectory is ever rendered and written to the target.

---

## `project.json` — Context Schema

Defines the variables that boilr collects from the user (or uses as defaults).

```json
{
    "Name":    "my-project",
    "Author":  "Your Name",
    "Year":    "2025",
    "License": ["MIT", "GNU GPL v3.0", "Apache 2.0"]
}
```

| Value type | Prompt behaviour |
|------------|-----------------|
| `string`   | Free-text input with default shown |
| `bool`     | Yes/No prompt (`y`, `yes`, `true` / `n`, `no`, `false`) |
| `[]string` | Numbered multiple-choice list; first item is default |
| `object`   | Nested recursive prompts |

---

## `template.Interface`

```go
type Interface interface {
    Execute(dirPrefix string) error
    UseDefaultValues()
    Info() Metadata
}
```

Implemented by `dirTemplate`.

---

## `template.Get(path)` — Factory

`Get` loads a template from the registry and returns an `Interface`:

1. Read and decode `project.json` → `Context map[string]interface{}`.
2. Read and decode `__metadata.json` if present → `Metadata`.
3. Build `FuncMap` (custom functions + Sprig).
4. Return `dirTemplate{Path, Context, FuncMap, Metadata}`.

---

## `dirTemplate.Execute(dirPrefix)` — Render Pipeline

```
Execute(dirPrefix)
  │
  ├─ BindPrompts()  OR  BindDefaults()
  │    └─ for each key in Context:
  │         prompt.New(key, defaultValue) → resolved value
  │
  └─ filepath.Walk(template/)
       for each node:
         │
         ├─ render node name with text/template
         │
         ├─ node is dir  →  os.MkdirAll(destDir)
         │
         └─ node is file
              ├─ isBinary()?
              │    yes → io.Copy (raw)
              │    no  → render content with text/template
              │
              └─ write to destDir
                   └─ if output is only whitespace → delete file
```

### Binary Detection (`isBinary`)

A file is treated as binary if:
- It contains a null byte (`\x00`), **or**
- It is not valid UTF-8.

Binary files are copied byte-for-byte; no template rendering is attempted.

### Ignored Files

Files matching `IgnoreCopyFiles` (`.DS_Store`, `Thumbs.db`, etc.) are skipped entirely.

---

## Prompt Binding

### `BindPrompts()` — Interactive Mode

For each key in `Context`:

1. Call `prompt.New(key, defaultValue)` to get a prompt closure.
2. The closure asks the user once and caches the answer.
3. The resolved map is used for all template renders in this execution.

### `BindDefaults()` — `--use-defaults` Mode

Context values stay as-is from `project.json`. No stdin interaction.

---

## Template Functions (`FuncMap`)

All of Go's standard `text/template` built-ins are available, plus:

### Custom Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `hostname` | `() string` | System hostname (`os.Hostname`) |
| `username` | `() string` | Current OS username |
| `toBinary` | `(n int) string` | Format integer as binary string |
| `formatFilesize` | `(bytes float64) string` | Human-readable size (KB/MB/GB…) |
| `toTitle` | `(s string) string` | Title-case a string |
| `password` | `(length, digits, symbols int, noUpper, allowRepeat bool) string` | Secure random password |
| `randomBase64` | `(length int) string` | Random base64-encoded string |

### Sprout Functions

All functions from [`go-sprout/sprout`](https://github.com/go-sprout/sprout) are available,
giving ~100 extra helpers: string manipulation, math, date/time, encoding, reflection, etc.
Sprout is the active successor to sprig with renamed canonical functions (e.g. `toKebabCase`,
`toSnakeCase`) and opt-in registries.

### Template Options

```go
tmpl.Option("missingkey=invalid")
```

Any variable referenced in a template that has no value in `Context` causes an error,
preventing silent empty substitutions.

---

## Metadata (`template.Metadata`)

```go
type Metadata struct {
    Tag        string
    Repository string
    Created    JSONTime
}
```

`JSONTime` is a custom type wrapping `time.Time` with RFC1123Z serialisation and a
human-readable `"X time ago"` display format for the `list` command.

---

## Validation

`validate.ValidateTemplate(path)`:

1. Confirm `template/` subdirectory exists inside path.
2. Call `testTemplate(path)` — execute with all defaults, discard output.
3. Return any render errors found.

This is used by both `boilr template validate` and the pre-save check in `boilr template save`.
