---
title: Overview
weight: 1
---

## Goals

- Fix all known correctness bugs from the original boilr v1
- Replace the prompt layer with huh (Charm)
- Replace output styling with lipgloss
- Use configurable template delimiters (default `{{ }}`)
- Replace `project.json` with `project.yml`
- Add hooks, `--values`/`--arg`, XDG config, `.specsverbatim`, conditional files
- Add computed values (post-prompt derived context keys)
- Add `specs use` one-step command
- Maintain backward compatibility with v1 templates

---

## Package Structure

```text
specs-cli/
├── main.go                       # main() — XDG init, cmd.Execute()
├── go.mod
└── internal/
    ├── specs/                    # global config & constants
    │   ├── configuration.go      # XDG paths, file name constants
    │   └── errors.go             # sentinel errors
    ├── cmd/                      # one file per Cobra command
    │   ├── root.go
    │   ├── app.go                # App struct, --debug/--safe-mode flags
    │   ├── use.go                # specs use <source> <target-dir>
    │   ├── template.go           # specs template subcommand group
    │   ├── template_download.go
    │   ├── template_save.go
    │   ├── template_use.go       # --values, --arg, --no-hooks
    │   ├── template_list.go
    │   ├── template_validate.go
    │   ├── template_rename.go
    │   ├── template_delete.go
    │   ├── template_update.go
    │   ├── template_upgrade.go
    │   └── version.go
    ├── registry/                 # on-disk template store operations
    │   └── registry.go           # Entry, UpgradeResult, Load(), Upgrade()
    ├── template/                 # template loading & execution engine
    │   ├── template.go           # Get(), Execute(), configurable delimiters
    │   ├── context.go            # project.yml parsing, LoadProjectFile(), referenced defaults, computed values
    │   ├── verbatim.go           # .specsverbatim loading & matching
    │   ├── functions.go          # FuncMap (custom + Sprout)
    │   ├── specsregistry.go      # custom Sprout registry (hostname, password, etc.)
    │   ├── analysis.go           # AST-based conditional variable analysis
    │   ├── cond.go               # Cond interface and implementations
    │   ├── metadata.go           # Metadata struct, JSONTime, LoadMetadata(), SaveMetadata()
    │   ├── status.go             # TemplateStatus — remote stale-check caching
    │   └── validate.go           # template validation helpers
    ├── hooks/                    # hook execution
    │   └── hooks.go              # Load(), Run(), context → env vars
    ├── host/                     # source URL parsing
    │   └── source.go             # github:user/repo, HTTPS URL, local path
    └── util/
        ├── exit/                 # exit codes
        ├── git/                  # go-git wrapper, SSH auth, remote check
        ├── osutil/               # file operations (CopyDir, etc.)
        ├── output/               # lipgloss-based logger + table renderer
        ├── validate/             # Name() validator and argument validators
        └── values/               # --values file (JSON/YAML) + --arg flag parsing
```

---

## CLI Command Tree

```text
specs [--version|-v]
      [--debug]                             enable debug output
      [--safe-mode]                         disable env/filesystem template functions + hooks
      [--no-env-prefix]                     disable SPECS_ prefix on hook env vars
      [--output/-o pretty|json]             output format (default: pretty)
│
├── use <source> <target-dir>               one-step, no registry entry
│     [--values file.yaml|json]
│     [--arg Key=Value]...
│     [--use-defaults]
│     [--no-hooks]
│
├── template
│   ├── download [--force] <source> <name>
│   ├── save     [--force] <path> <name>
│   ├── use      <name> <target-dir>
│   │     [--values file.yaml|json]
│   │     [--arg Key=Value]...
│   │     [--use-defaults]
│   │     [--no-hooks]
│   ├── list|ls
│   ├── update   [name]                     refresh status cache (all if no name)
│   ├── upgrade  [name]                     re-clone to latest (all if no name)
│   ├── delete|remove|rm|del <name>...
│   ├── validate <path>
│   └── rename|mv <old> <new>
│
├── init    [--force]
└── version [--dont-prettify]
```

### `specs use <source> <target-dir>`

One-step command — downloads, executes, discards. No registry entry created.

| Format                | Example                                               |
|-----------------------|-------------------------------------------------------|
| GitHub shorthand      | `github:Ilyes512/boilr-laravel-project`               |
| GitHub with branch    | `github:Ilyes512/boilr-laravel-project:main`          |
| Full HTTPS URL        | `https://github.com/Ilyes512/boilr-laravel-project`   |
| SCP-style SSH         | `git@github.com:Ilyes512/boilr-laravel-project`       |
| SSH URL               | `ssh://git@github.com/Ilyes512/boilr-laravel-project` |
| Local path (explicit) | `file:./my-template`                                  |
| Local path (implicit) | `./my-template` or `/absolute/path`                   |

**Source validation rules** (enforced at parse time, before any network call):

- **GitHub shorthand** — owner and repo must match GitHub's naming rules: alphanumeric, dots,
  hyphens, underscores; must start and end with an alphanumeric character; max 39 chars for
  owner, 100 for repo; exactly one `/` separator. Branch (if given) must be non-empty, contain
  no whitespace, and must not include `..`.
- **HTTPS / SSH URLs** — must have a non-empty host and a path with at least two non-empty
  segments (`/owner/repo`).

SSH clones are authenticated automatically via SSH agent or standard key files
(`~/.ssh/id_ed25519`, `id_rsa`, `id_ecdsa`). Host key verification uses `~/.ssh/known_hosts`.

---

## Template Structure

```text
<template-root>/
├── project.yml              # variable schema, defaults, optional inline hooks
├── .specsverbatim            # verbatim-copy glob patterns (opt-out from rendering)
├── __metadata.json           # written by specs on download/save
├── __status.json             # remote status cache (written by specs template list)
├── hooks/                    # script-based hooks (mutually exclusive with hooks: in project.yml)
│   ├── pre-use.sh
│   └── post-use.sh
└── template/
    ├── {{ if .UseSonarQube }}sonar-project.properties{{ end }}
    ├── composer.json
    ├── composer.lock         # matched by .specsverbatim → verbatim copy
    └── .github/
        └── workflows/
            └── ci.yml        # configure __delimiters: "[[ ]]" to pass ${{ }} through untouched
```

---

## Configuration

```text
$XDG_CONFIG_HOME/specs/          (default: ~/.config/specs/)
└── templates/
    └── <name>/
        ├── project.yml
        ├── .specsverbatim
        ├── __metadata.json
        ├── __status.json
        ├── hooks/
        └── template/
```

---

## Data Flow — `specs template use`

```mermaid
flowchart TD
    A[validate args & flags] --> B[check registry initialised + name exists]
    B --> C["template.Get(registry/name)\nparses project.yml, resolves referenced defaults,\nloads .specsverbatim, analyses AST for conditionals"]
    C --> D["hooks.Load(templateRoot, rawConfig)"]
    D --> E[merge --values + --arg overrides into context]
    E --> F["huh form: iterative prompting\n(unconditional vars first, then conditional by dependency layer)"]
    F --> G["ApplyComputed — resolve computed: values post-prompt"]
    G --> H["hooks.Run(pre-use) if defined"]
    H --> I["Execute(tmpDir) — render template/ into temp dir"]
    I --> J["osutil.CopyDir(tmpDir → targetDir)"]
    J --> K["hooks.Run(post-use, env=SPECS_-prefixed context)"]
    K --> L[output success]
```

---

## Data Flow — `specs use <source> <target>`

```mermaid
flowchart TD
    A[parse source format] --> B{source type?}
    B -->|github shorthand / URL| C["git.Clone(tmpDir, url)"]
    B -->|local path| D["copy local path to tmpDir"]
    C & D --> E["template.Get(tmpDir)"]
    E --> F[same flow as specs template use]
    F --> G[discard tmpDir — no registry entry]
```

---

## Template Execute — File Walk

```mermaid
flowchart TD
    A["filepath.WalkDir(template/)"] --> B{ignoredFile?}
    B -->|yes| Skip1[skip]
    B -->|no| C[render path as template]
    C --> D{render error or result empty?}
    D -->|yes| Skip2[skip]
    D -->|no| E{any path segment empty?}
    E -->|yes| Skip3[skip dir tree]
    E -->|no| F{is directory?}
    F -->|yes| Mkdir[os.MkdirAll]
    F -->|no| G{matches .specsverbatim?}
    G -->|yes| Copy1[copy verbatim]
    G -->|no| H{isBinary?}
    H -->|yes| Copy2[copy verbatim]
    H -->|no| I[render content as template]
    I --> J{whitespace-only result?}
    J -->|yes| Skip4[skip — do not create file]
    J -->|no| Write[write to dest]
```

---

## Context Resolution

```mermaid
flowchart TD
    A[load project.yml] --> B["strip computed: and hooks: sections from user input map"]
    B --> C["resolve referenced defaults\n(topological sort on template expressions in string defaults)"]
    C --> D[merge --values file overrides]
    D --> E[merge --arg flag overrides]
    E --> F["iterative prompting via huh\n(unreferenced variables skipped entirely)"]
    F --> G["resolve computed: values post-prompt\n(topological sort; each result merged before next)"]
    G --> H["run hooks with full context\n(user inputs + computed values)"]
    H --> I["Execute — render template files"]
```

---

## Error Handling (`internal/specs/errors.go`)

Sentinel errors are declared in `internal/specs/errors.go` and should always be wrapped with `%w`
so that callers can use `errors.Is` to distinguish them:

| Sentinel                     | Kind string                 | Raised when                                                                                            |
|------------------------------|-----------------------------|--------------------------------------------------------------------------------------------------------|
| `ErrTemplateNotFound`        | `template_not_found`        | Named template does not exist in the registry                                                          |
| `ErrTemplateAlreadyExists`   | `template_already_exists`   | Template name is already in use (save/download/rename without `--force`)                               |
| `ErrTemplateDirMissing`      | `template_dir_missing`      | Template root exists but has no `template/` subdirectory                                               |
| `ErrBothHookSources`         | `both_hook_sources`         | Both inline hooks and a `hooks/` directory are present                                                 |
| `ErrAmbiguousProjectFile`    | `ambiguous_project_file`    | Both `project.yaml` and `project.yml` exist in the template root                                       |
| `ErrInvalidDelimiters`       | `invalid_delimiters`        | `__delimiters` in `project.yaml` is malformed                                                          |
| `ErrProjectFileMissing`      | `project_file_missing`      | No `project.yaml`, `project.yml`, or `project.json` found                                              |
| `ErrLocalSource`             | `local_source`              | Local path given to a command that requires a remote URL                                               |
| `ErrInvalidComputedDef`      | `invalid_computed_def`      | `computed:` entry in `project.yaml` has wrong type, value type mismatch, or key conflict               |
| `ErrCyclicDependency`        | `cyclic_dependency`         | Cycle detected among computed or referenced-default keys                                               |
| `ErrInvalidSpecsVersion`     | `invalid_specs_version`     | `__specs__version` in `project.yaml` is not a string or not a parseable semver constraint              |
| `ErrSpecsVersionUnsatisfied` | `specs_version_unsatisfied` | Running CLI version does not satisfy the template's `__specs__version` constraint                      |
| `ErrReservedVariableName`    | `reserved_variable_name`    | A variable or computed name uses the reserved `__` prefix without being a recognised configuration key |

`specs.KindOf(err error) string` returns the stable kind string for any error in the chain,
or `""` when no known sentinel is wrapped.

---

## Output System (`internal/util/output`)

All user-facing output goes through the `output.Writer` interface:

```go
type Writer interface {
    Info(format string, args ...any)
    Warn(format string, args ...any)
    Error(format string, args ...any)
    // WriteErr renders err as an error message; JSON output includes error_kind when known.
    WriteErr(err error)
    Table(headers []string, rows [][]string)
}
```

Two implementations are selected at startup via `--output`:

| Flag value         | Writer        | Behaviour                                                                                        |
|--------------------|---------------|--------------------------------------------------------------------------------------------------|
| `pretty` (default) | `HumanWriter` | Lipgloss-styled text; `Info` → stdout, `Warn`/`Error`/`WriteErr` → stderr                        |
| `json`             | `JSONWriter`  | NDJSON lines; `Table` → JSON array to stdout; `WriteErr` adds `"error_kind"` for known sentinels |

`JSONWriter.WriteErr` example for a known sentinel:

```json
{"level":"error","message":"template not found: mytemplate","error_kind":"template_not_found"}
```

The `JSONWriter` is useful for scripting (`specs template list --output json`) or CI pipelines
that parse structured output.

---

## Logging

`specs` uses `log/slog` for structured diagnostic output. All packages emit logs via the
package-level `slog.Debug/Info/Warn/Error` functions, which route through the global default
logger. `NewApp()` calls `slog.SetDefault` to install a text handler at `Info` level;
`PersistentPreRunE` re-sets it when `--debug --output=json` swaps the handler to JSON.

`slog` is a **debug-only diagnostic channel** on **stderr** — it is silent on a normal run (every
log point is `Debug`, suppressed at the default `Info` level) and is distinct from the two
`output.Writer` formats (`pretty`/`json`) that produce user-facing output on stdout. Do not use
`slog` for user-facing reporting; use `output.Writer`.

### Flags

| Flag                        | Effect                                                                                                                        |
|-----------------------------|-------------------------------------------------------------------------------------------------------------------------------|
| `--debug`                   | Raises the slog level from `Info` to `Debug`; all debug logs become visible                                                   |
| `--output=json` + `--debug` | Swaps the slog handler to `slog.NewJSONHandler` writing to stderr; debug logs are emitted as NDJSON distinct from stdout data |

### Log points

| Package             | Function             | Level | Attributes                                                                                 |
|---------------------|----------------------|-------|--------------------------------------------------------------------------------------------|
| `internal/template` | `Get`                | Debug | `template`, `keys`, `computed`                                                             |
| `internal/template` | `Execute`            | Debug | `path`, `dest`, `action` (render/verbatim/skip)                                            |
| `internal/template` | `Execute`            | Debug | `template`, `dest`, `rendered`, `verbatim`, `skipped` (summary)                            |
| `internal/template` | `ApplyComputed`      | Debug | `key`, `source`="computed"                                                                 |
| `internal/hooks`    | `Hooks.Run`          | Debug | `trigger`, `commands`, `command`                                                           |
| `internal/cmd`      | `executeTemplate`    | Debug | `key`, `source` (values_file/arg_flag/default/prompt) — one log per key, final source only |
| `internal/registry` | `Upgrade`            | Debug | `template`, `repo`, `branch`, `target_ref`, `latest_version`                               |
| `internal/util/git` | `Clone`              | Debug | `repo`, `dest`, `branch` (start and complete)                                              |
| `internal/util/git` | `Describe`           | Debug | `dest`, `commit`, `version` (or `error` on failure)                                        |
| `internal/util/git` | `CheckRemoteContext` | Debug | `repo`, `branch`, `dest`, `up_to_date`, `latest_version`, `error_kind`                     |

### Consistent attribute keys

| Key        | Meaning                                                                                   |
|------------|-------------------------------------------------------------------------------------------|
| `template` | Registered template name (e.g. `"minimal"`) — primary user identifier                     |
| `path`     | Template-relative source path of a file (e.g. `"src/foo.go"`)                             |
| `dest`     | Absolute destination path on the filesystem                                               |
| `repo`     | Remote repository URL                                                                     |
| `branch`   | Git branch or tag ref                                                                     |
| `commit`   | Full git commit SHA                                                                       |
| `version`  | git-describe-style version string                                                         |
| `trigger`  | Hook trigger name (`pre-use`, `post-use`)                                                 |
| `key`      | Context variable name                                                                     |
| `source`   | How a context value was provided (`default`/`prompt`/`values_file`/`arg_flag`/`computed`) |
| `action`   | File decision (`render`/`verbatim`/`skip`)                                                |
| `error`    | Underlying error (formatted as `%v`)                                                      |

### Example: structured debug output

```sh
# Text handler (default with --debug):
specs --debug template use minimal ./out

# NDJSON handler (--debug + --output=json):
specs --debug --output=json template ls 2>debug.ndjson
```

`--output=json` controls the **data** format on stdout; `--debug` + `--output=json` controls
the **log** format on stderr. The two streams are independent. `slog.SetDefault` is called
by `NewApp()` and re-set in `PersistentPreRunE` when `--debug --output=json` swaps the handler.

---

## Hooks Execution

```go
// internal/hooks/hooks.go

type Hooks struct {
    PreUse    []string // each entry: single command or multiline bash script
    PostUse   []string
    EnvPrefix string   // prefix prepended to each context key in the env (e.g. "SPECS_")
}

// Load reads hook definitions from templateRoot.
// Sources (mutually exclusive — error if both are present):
//   - Inline: the "hooks" key in projectConfig (parsed from project.yml)
//   - Directory: hooks/pre-use.sh and hooks/post-use.sh under templateRoot
func Load(templateRoot string, projectConfig map[string]any, envPrefix string) (*Hooks, error)

// Run executes each command via bash -c.
// ctx is injected as SPECS_-prefixed uppercase env vars: ProjectName → SPECS_PROJECTNAME.
// {{ }} expressions in hook commands are rendered against ctx before execution.
// Stops and returns error on first non-zero exit.
func (h *Hooks) Run(trigger, cwd string, ctx map[string]any, funcMap template.FuncMap, delims specs.Delimiters) error
```

---

## Packages Added / Changed vs boilr v1

| Package                  | Status      | Change                                                                                                                                                                          |
|--------------------------|-------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `internal/specs`         | **new**     | XDG paths, file name constants, sentinel errors, `KindOf()` (replaces `pkg/boilr`)                                                                                              |
| `internal/registry`      | **new**     | on-disk template store: `Entry`, `Load()`, `Upgrade()`                                                                                                                          |
| `internal/cmd`           | updated     | new `use.go`, `template_update.go`, `template_upgrade.go`, iterative conditional prompting; no longer reads project files or `__metadata.json` directly                         |
| `internal/template`      | updated     | configurable delimiters (default `{{ }}`), `context.go`, `verbatim.go`, conditional skip, AST analysis, status; exports `LoadProjectFile()`, `LoadMetadata()`, `SaveMetadata()` |
| `internal/hooks`         | **new**     | hook loading and execution                                                                                                                                                      |
| `internal/util/output`   | updated     | lipgloss-based logger + table renderer; `WriteErr` with JSON `error_kind` for known sentinels (replaces tlog + tabular)                                                         |
| `internal/util/values`   | **new**     | `--values` file (JSON/YAML) and `--arg` flag parsing                                                                                                                            |
| `internal/host`          | updated     | source format parsing (github:, HTTPS, SSH, local path)                                                                                                                         |
| `pkg/prompt`             | **removed** | replaced by `huh`                                                                                                                                                               |
| `pkg/util/tlog`          | **removed** | replaced by `internal/util/output`                                                                                                                                              |
| `pkg/util/tabular`       | **removed** | replaced by `internal/util/output`                                                                                                                                              |
| `pkg/util/exec`          | **removed** | no longer needed (hooks use `os/exec` directly)                                                                                                                                 |
| `internal/util/exit`     | unchanged   |                                                                                                                                                                                 |
| `internal/util/git`      | updated     | SSH auth, `CheckRemoteContext()` (context-aware), `Describe()` for status tracking; `RemoteCheckResult.Err()` returns typed sentinel errors                                     |
| `internal/util/osutil`   | updated     | `CopyDir()` recursive copy                                                                                                                                                      |
| `internal/util/validate` | updated     | `Name()` validator (alphanumeric + hyphens + underscores)                                                                                                                       |
