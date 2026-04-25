# Boilr v2 — Architecture

## Goals

- Fix all known correctness bugs from v1
- Replace the prompt layer with huh (Charm)
- Replace output styling with lipgloss + bubbles
- Switch template delimiters to `[[ ]]`
- Replace `project.json` with `project.yaml`
- Add hooks, `--values`/`--arg`, XDG config, `.boilrignore`, conditional files
- Add computed values (post-prompt derived context keys)
- Add `boilr use` one-step command
- Maintain backward compatibility with v1 templates

---

## Package Structure

```
boilr/
├── boilr.go                      # main() — XDG init, cmd.Run()
├── go.mod
└── pkg/
    ├── boilr/                    # global config & constants
    │   ├── configuration.go      # XDG paths, file names, ignored files
    │   └── errors.go             # sentinel errors
    ├── cmd/                      # one file per Cobra command
    │   ├── root.go
    │   ├── use.go                # NEW: boilr use github:user/repo .
    │   ├── download.go
    │   ├── save.go
    │   ├── template_use.go       # updated: --values, --arg, --no-hooks
    │   ├── list.go
    │   ├── delete.go
    │   ├── validate.go
    │   ├── rename.go
    │   ├── init.go
    │   ├── version.go
    │   ├── flags.go
    │   ├── metadata.go
    │   └── must_validate.go      # updated: AlphanumericExt for tags
    ├── template/                 # template loading & execution engine
    │   ├── template.go           # Get(), Execute(), [[ ]] delimiters
    │   ├── context.go            # NEW: project.yaml parsing, referenced defaults
    │   ├── verbatim.go           # NEW: .specsverbatim loading & matching
    │   ├── functions.go          # FuncMap (custom + Sprout)
    │   └── metadata.go           # Metadata struct, JSONTime
    ├── hooks/                    # NEW: hook execution
    │   └── hooks.go              # Load(), Run(), context → env vars
    ├── host/                     # GitHub URL helpers (unchanged)
    │   └── github.go
    └── util/
        ├── exec/                 # subprocess runner (unchanged)
        ├── exit/                 # exit codes (unchanged)
        ├── git/                  # go-git wrapper (unchanged)
        ├── osutil/               # file operations (unchanged)
        ├── output/               # NEW: replaces tlog + tabular
        │   ├── log.go            # lipgloss-based levelled logger
        │   └── table.go          # bubbles table renderer
        ├── stringutil/           # io.ReadWriter over string (unchanged)
        └── validate/             # argument validators
            └── ...               # updated: tags use AlphanumericExt
```

---

## CLI Command Tree

```
boilr
├── use <source> <target-dir>               NEW — one-step, no registry entry
│     [--values file.yaml]
│     [--arg Key=Value]...
│     [--use-defaults]
│     [--no-hooks]
│
├── template
│   ├── download [--force] <repo> <name>     updated: supports repo:branch
│   ├── save     [--force] <path> <name>
│   ├── use      <name> <target-dir>         updated: --values, --arg, --no-hooks
│   │     [--values file.yaml]
│   │     [--arg Key=Value]...
│   │     [--use-defaults]
│   │     [--no-hooks]
│   ├── list     [--dont-prettify]
│   ├── delete   <name>...
│   ├── validate <path>
│   └── rename   <old> <new>
│
├── init    [--force]
└── version [--dont-prettify]
```

### `boilr use <source> <target-dir>`

One-step command. Source formats:

| Format | Example |
|--------|---------|
| GitHub shorthand | `github:Ilyes512/boilr-laravel-project` |
| GitHub with branch | `github:Ilyes512/boilr-laravel-project:main` |
| Full HTTPS URL | `https://github.com/Ilyes512/boilr-laravel-project` |
| Local path | `file:./my-template` or just `./my-template` |

Downloads to a temp directory, executes, discards. No registry entry created.

---

## Template Structure (v2)

```
<template-root>/
├── project.yaml              # variable schema, defaults, optional inline hooks
├── .boilrignore              # verbatim-copy glob patterns
├── __metadata.json           # written by boilr on download/save
├── hooks/                    # optional script-based hooks (mutually exclusive with
│   ├── pre-use.sh            #   hooks: key in project.yaml)
│   └── post-use.sh
└── template/
    ├── [[if .UseSonarQube]]sonar-project.properties[[end]]
    ├── [[if .UseSonarQube]]docs/images[[end]]/
    │   └── badge.png
    ├── composer.json
    ├── composer.lock         # matched by .boilrignore → verbatim
    ├── package-lock.json     # matched by .boilrignore → verbatim
    └── .github/
        └── workflows/
            └── ci.yml        # ${{ }} passes through untouched
```

---

## Configuration (v2)

```
$XDG_CONFIG_HOME/boilr/          (default: ~/.config/boilr/)
├── config.yaml                  # optional user config
└── templates/
    └── <name>/
        ├── project.yaml
        ├── .boilrignore
        ├── __metadata.json
        ├── hooks/
        └── template/
```

Resolved via `github.com/adrg/xdg`:
```go
configDir, _ := xdg.ConfigFile("boilr")          // config.yaml location
templateDir  := filepath.Join(configDir, "templates")
```

---

## Key Data Flows

### `boilr template use <name> <target-dir>`

```
validate args & flags
  │
check registry initialised + name exists
  │
template.Get(registryPath/name)
  ├── parse project.yaml (fallback: project.json)
  ├── resolve referenced defaults (topological sort on [[ ]] in default values)
  ├── load .boilrignore patterns
  └── parse hooks definition (inline OR hooks/ directory, not both)
  │
merge --values file + --arg overrides into context
  │
hooks.Run("pre-use", cwd=templateSourceDir)    ← abort if non-zero exit
  │
huh form — prompt for each key not already provided
  │
Execute(tmpDir)
  └── filepath.Walk(template/)
        for each node:
          ├── ignoreCopyFile?       → skip
          ├── render name with [[ ]] engine
          │     ├── parse/exec error → skip + warn
          │     ├── rendered == ""   → skip
          │     └── any segment == ""→ skip
          ├── directory             → mkdir
          └── file
                ├── .boilrignore match? → copy verbatim
                ├── isBinary?           → copy verbatim
                └── text                → render content with [[ ]] engine
                                            → delete if whitespace-only
  │
osutil.CopyRecursively(tmpDir, targetDir)
  │
hooks.Run("post-use", cwd=targetDir, env=resolvedContext)
  │
output success
```

### `boilr use github:user/repo .`

```
parse source → determine: github shorthand / URL / local path
  │
git.Clone(tmpDir, url)          ← or: copy local path
  │
template.Get(tmpDir)
  │
[same flow as template use from here]
  │
discard tmpDir (no registry entry)
```

---

## Context Resolution (v2)

Context is built in four stages before prompting:

```
1. Load project.yaml defaults
       │
2. Strip computed: section (not user inputs — resolved post-prompt)
       │
3. Resolve referenced defaults ([[ ]] in default values, topological order)
       │
4. Merge --values file   (provided keys override defaults; error if key is computed)
       │
5. Merge --arg flags     (take precedence over --values; error if key is computed)
       │
6. Prompt for remaining keys (huh form, skipped for pre-provided keys)
       │         ↓ final user context
7. Resolve computed values (topological sort; each result merged before next)
       │         ↓ full context (user inputs + computed values)
8. Execute template files + hook commands
```

See [11-computed-values.md](../11-computed-values.md) for the full design.

---

## Hooks Execution

```go
// pkg/hooks/hooks.go

type Hooks struct {
    PreUse  []string  // each entry: single command or multiline script
    PostUse []string
}

// Load reads hooks from project.yaml inline definition OR hooks/ directory.
// Returns error if both are present.
func Load(templateRoot string, projectYAML *ProjectConfig) (*Hooks, error)

// Run executes each command in sequence via `bash -c`.
// ctx is injected as uppercase env vars: ProjectName → PROJECTNAME.
// Stops and returns error on first non-zero exit.
func (h *Hooks) Run(trigger string, cwd string, ctx map[string]any) error
```

Commands in the list may contain `[[ ]]` template expressions — resolved against the
final context before execution.

---

## Packages Added / Changed vs v1

| Package | Status | Change |
|---------|--------|--------|
| `pkg/boilr` | updated | XDG paths, yaml file name constant |
| `pkg/cmd` | updated | new `use.go`, updated `template_use.go`, name validator fix |
| `pkg/template` | updated | `[[ ]]` delimiters, `context.go`, `verbatim.go`, conditional skip |
| `pkg/hooks` | **new** | hook loading and execution |
| `pkg/util/output` | **new** | lipgloss logger + bubbles table (replaces tlog + tabular) |
| `pkg/prompt` | **removed** | replaced by `huh` |
| `pkg/util/tlog` | **removed** | replaced by `pkg/util/output` |
| `pkg/util/tabular` | **removed** | replaced by `pkg/util/output` |
| `pkg/host` | unchanged | |
| `pkg/util/exec` | unchanged | |
| `pkg/util/exit` | unchanged | |
| `pkg/util/git` | unchanged | |
| `pkg/util/osutil` | unchanged | |
| `pkg/util/stringutil` | unchanged | |
| `pkg/util/validate` | unchanged | |
