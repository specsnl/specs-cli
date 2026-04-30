# Boilr v2 — Template Engine

## Decision

Retain **Go `text/template` + Sprig** as the rendering engine with two targeted changes:

1. **Change delimiters** from `{{ }}` to `[[ ]]`
2. **Add verbatim copy via `.boilrignore`** (opt-out from template rendering)

No new template engine or scripting language is introduced. The engine itself is not the problem.

---

## 1. Delimiter Change: `{{ }}` → `[[ ]]`

### Problem

Go templates and GitHub Actions both use `{{ }}`. Every GitHub Actions expression in a
template file must currently be wrapped in a backtick raw string:

```yaml
# Today — painful
group: {{`${{ github.workflow }}-${{ github.ref }}`}}
SONAR_TOKEN: {{`${{ secrets.SONAR_TOKEN }}`}}
```

This affects every workflow file in a template, making them hard to read and error-prone
to author.

### Solution

Switch to `[[ ]]` delimiters via Go's built-in `template.Delims()`:

```go
template.New(name).Delims("[[", "]]").Funcs(funcMap).Parse(content)
```

### Result

```yaml
# v2 — clean
group: ${{ github.workflow }}-${{ github.ref }}
SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}

# Template expressions use [[ ]]
MARIADB_DATABASE: "[[toSnakeCase .ProjectShortName]]_test"
```

All existing template features work identically — variable substitution, conditionals,
pipes, Sprig functions — only the delimiters change.

`[[ ]]` is already a recognised convention for this purpose: Angular uses it to avoid
conflicts with server-side `{{ }}` templating, and Ansible uses it as a Jinja2 override
for the same reason.

### Migration of existing templates

A one-time sed-style substitution converts v1 templates to v2:
- Replace `{{` → `[[` and `}}` → `]]` in template expressions
- Remove all `{{`` ` ``}}` backtick escape wrappers — the content inside becomes plain text

---

## 2. Verbatim Copy (Per-File Template Opt-Out)

### Problem

The current engine attempts to render every non-binary file as a Go template. Some files
should never be parsed:

- `composer.lock`, `package-lock.json` — large dependency lockfiles, no variables
- Any file where the content coincidentally contains `[[ ]]` syntax (e.g. Angular source)
- Binary-adjacent formats that pass the UTF-8 check but contain no template expressions

Currently this relies on a binary-detection heuristic (null bytes / invalid UTF-8). That
heuristic is not appropriate for text files that simply should not be templated.

### Solution

A `.boilrignore` file at the template root (alongside `project.json`) lists glob patterns
for files that should be copied verbatim:

```
# .boilrignore
composer.lock
package-lock.json
*.min.js
vendor/**
```

Files matching any pattern are copied byte-for-byte without any template parsing.

---

## 3. Conditional Files and Directories

### Problem

The only way to conditionally exclude a file today is to wrap its entire content in an
`[[if .Condition]]...[[end]]` block. When the condition is false the file becomes
whitespace-only and boilr deletes it. This is awkward, unreadable, and impossible for
binary files. There is no way to conditionally exclude a whole directory.

### Solution

Use the filename (or directory name) itself as the template expression. After rendering,
boilr checks the result:

- If the rendered name is **empty or whitespace** → skip (do not create the file or
  directory).
- If any **path segment** of the rendered name is empty or whitespace → skip the file.

This second rule is what makes entire directory trees work: files inside a conditionally
excluded directory inherit its empty segment and are skipped automatically.

### How it works

```
filepath.Walk(template/)
  for each node:
    1. Render the full relative path with the template engine
    2. strings.TrimSpace(renderedName)
    3. If result == ""  →  return nil (skip)
    4. Split result on os.PathSeparator
    5. For each segment: if strings.TrimSpace(segment) == ""  →  return nil (skip)
    6. Otherwise proceed to create file or directory
```

### Examples

**Conditional file** — skip a file when `UseSonarQube` is false:

```
template/
└── [[if .UseSonarQube]]sonar-project.properties[[end]]
```

| `UseSonarQube` | Rendered name | Result |
|---|---|---|
| `true` | `sonar-project.properties` | file created |
| `false` | `` (empty) | skipped |

**Conditional directory** — skip a directory and all its contents:

```
template/
└── [[if .UseSonarQube]]docs/images[[end]]/
    ├── coverage-badge.png
    └── analysis-chart.png
```

When `UseSonarQube` is false:

| Node | `oldName` (template) | Rendered | Verdict |
|---|---|---|---|
| dir | `[[if .UseSonarQube]]docs/images[[end]]` | `` | empty → skipped |
| file | `[[if .UseSonarQube]]docs/images[[end]]/coverage-badge.png` | `/coverage-badge.png` | segment `""` before `/` → skipped |
| file | `[[if .UseSonarQube]]docs/images[[end]]/analysis-chart.png` | `/analysis-chart.png` | segment `""` before `/` → skipped |

### Filename template errors

If the filename template itself fails to parse or execute (e.g. malformed syntax), the
file is skipped and a debug log line is emitted. This replaces the previous behaviour of
`template.Must` which caused a hard panic.

### Relation to content-level conditionals

Both approaches remain valid and can coexist:

| Technique | When to use |
|---|---|
| Filename template `[[if .X]]name[[end]]` | Entire file or directory is conditional |
| Content block `[[if .X]]...[[end]]` | Part of a file is conditional; rest is always emitted |

For binary files (images, fonts, compiled assets) only the filename approach works — binary
content cannot contain template syntax.

---

## Updated Template Structure

```
<template-root>/
├── project.yaml          # variable schema & defaults  (RENAMED from project.json)
├── .boilrignore          # verbatim-copy glob patterns (NEW)
├── __metadata.json       # written by boilr            (unchanged)
├── hooks/                # optional pre/post scripts   (NEW, see plan 07)
│   ├── pre-use.sh
│   └── post-use.sh
└── template/
    ├── composer.lock     # matched by .boilrignore → copied verbatim
    ├── package-lock.json # matched by .boilrignore → copied verbatim
    ├── composer.json     # rendered with [[ ]] delimiters
    ├── README.md         # rendered with [[ ]] delimiters
    └── .github/
        └── workflows/
            └── ci.yml    # rendered; ${{ }} passes through untouched
```

**Backward compatibility:** if `project.yaml` is not found, boilr falls back to `project.json`
so existing v1 templates continue to work without changes.

---

## Render Pipeline (updated)

```
Execute(targetDir)
  │
  ├─ BindPrompts() / BindDefaults()
  ├─ Load .boilrignore patterns
  │
  └─ filepath.Walk(template/)
       for each node:
         │
         ├─ ignoreCopyFile()?          → skip (.DS_Store etc.)
         ├─ render filename template
         │    ├─ parse error           → skip + debug log
         │    ├─ rendered == ""        → skip
         │    └─ any path segment == ""→ skip
         │
         ├─ node is directory          → mkdir
         │
         └─ node is file
              ├─ matches .boilrignore? → copy verbatim
              ├─ isBinary()?           → copy verbatim
              └─ text file             → render content with [[ ]] engine
                                           ├─ parse/execute error → copy verbatim + debug log
                                           └─ output whitespace-only → delete file
```

---

## `project.yaml` format

Replaces `project.json`. Same schema, more readable, supports comments.

```yaml
# project.yaml

ProjectName: My Acme Project
ProjectShortName: acme-12
ProjectDescription: A Laravel project scaffolded with boilr.

# Select — first value is the default
PhpVersion:
  - "8.5"
  - "8.4"
  - "8.3"

RepoName: acme/project

# Select
ComposerLicense:
  - MIT
  - proprietary

IssuePrefix: ACM

# Bool — false = no, true = yes
UseSonarQube: false

# Referenced default — resolved after ProjectShortName is answered
ProjectSlug: "[[toKebabCase .ProjectShortName]]"

# Hooks inline (mutually exclusive with hooks/ directory)
# Each hook is a list; items run sequentially via bash -c
# Use | for multiline scripts
hooks:
  post-use:
    - composer install
    - npm install
    - |
      git init
      git add -A
      git commit -m "Initial commit: [[.ProjectName]]"
```

**Type coercion gotcha:** always quote strings that look like numbers.
`8.4` → parsed as `float64`. `"8.4"` → parsed as `string`. PHP versions, semver strings,
and numeric-looking IDs must be quoted.

---

## What Does Not Change

| Feature | Status |
|---------|--------|
| `project.json` variable schema | Unchanged |
| Prompt types (string, bool, select, multiselect) | Unchanged |
| Sprout functions (renamed: `toKebabCase`, `toSnakeCase`, etc.) | Replaced — see library decisions |
| Custom functions (`password`, `hostname`, etc.) | Unchanged |
| Conditional sections (`[[if .UseSonarQube]]`) | Unchanged, new delimiters |
| Whitespace-only file deletion | Unchanged |
| Binary file detection | Unchanged (heuristic retained as fallback) |
