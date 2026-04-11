# Specs CLI — Implementation Plan

## Project

- **Binary:** `specs`
- **Module:** `github.com/specsnl/specs-cli`
- **Purpose:** General-purpose developer CLI. `template` is the first subcommand group,
  replacing boilr. Future subcommand groups will extend the CLI without touching template code.
- **Command tree:** `specs template ...` for all template management subcommands
- **One-step command:** `specs use <source> <target>`

---

## Planned packages

These packages will be added to `go.mod` as they are needed during implementation — **not all
at once upfront**. This list is a reference so you know what to reach for in each phase.

| Package | Used for |
|---------|----------|
| `github.com/spf13/cobra` | CLI framework, subcommands, flags |
| `charm.land/huh/v2` | Interactive prompts for template variables |
| `charm.land/lipgloss/v2` | Terminal styling and coloured output |
| `charm.land/bubbles/v2` | Table component for `specs template list` |
| `gopkg.in/yaml.v3` | Parsing `project.yaml` |
| `github.com/go-sprout/sprout` | Extended template functions (date, crypto, strings, …) |
| `github.com/go-git/go-git/v5` | Cloning repositories |
| `github.com/adrg/xdg` | Resolving XDG config and data directories |
| `github.com/danwakefield/fnmatch` | Glob matching for `.specsverbatim` |
| `github.com/sethvargo/go-password` | Password generation template function |
| `github.com/docker/go-units` | Human-readable file size formatting |

## Branch strategy

Fresh repository — no legacy code. Each phase results in a buildable, testable state
before moving on.

---

## Coding conventions

- Use `any` instead of `interface{}` everywhere — they are identical but `any` is the modern idiomatic form since Go 1.18.
- Template delimiters are defined as `specs.DelimLeft` / `specs.DelimRight` — never hardcode `"[["` or `"]]"` in call sites.

---

## Testing requirement

Every phase includes tests alongside the implementation. No phase is considered done
without passing tests. Aim for:

- **Unit tests** for all pure functions (context resolution, ignore matching, URL parsing, validators).
- **Integration tests** for the template engine: use a real `project.yaml` + `template/` directory on disk.
- **Command tests** for CLI commands using Cobra's `ExecuteC()` with a captured output buffer.
- Test files live next to the code they test (`*_test.go` in the same package).

---

## Phase 1 — Project skeleton

**Goal:** Compilable entry point with bare Cobra root command.
**You learn:** Module setup, Cobra basics.
**Tests:** Verify `specs --help` exits 0 and prints expected usage text.

- `go mod init github.com/specsnl/specs-cli`
- `go get github.com/spf13/cobra`
- `main.go` — `main()` calls `cmd.Execute()`
- `pkg/cmd/root.go` — root Cobra command, `version` stub
- `pkg/cmd/version.go` — print version string, `--dont-prettify` flag

---

## Phase 2 — Config & output infrastructure

**Goal:** XDG paths resolved; styled terminal output working.
**You learn:** `adrg/xdg`, `lipgloss`.
**Tests:** Config path resolution (override `XDG_CONFIG_HOME` via env); output log functions
produce non-empty strings at each level.

- `pkg/specs/configuration.go` — XDG config dir, template dir path, file name constants
  (`project.yaml`, `__metadata.json`, `.specsverbatim`)
- `pkg/specs/errors.go` — sentinel errors
- `pkg/util/exit/` — exit codes
- `pkg/util/output/log.go` — lipgloss-based levelled logger (info, warn, error, debug styles)
- `pkg/util/output/table.go` — bubbles/lipgloss table renderer (used by `specs template list`)

---

## Phase 3 — Template engine

**Goal:** Load a `project.yaml`, render files with `[[ ]]` delimiters.
**You learn:** `go-yaml v3`, `text/template` custom delimiters, `sprout` registries.
**Tests:**
- Context parsing: string, bool, select, referenced default (topological sort), fallback to `project.json`.
- Verbatim matching: patterns in `.specsverbatim` match correct file paths.
- Execute: table-driven tests covering conditional filenames (true/false), verbatim copy
  (specsverbatim + binary), whitespace-only deletion, nested conditional directories.

Files:

- `pkg/template/metadata.go` — `Metadata` struct, `JSONTime`
- `pkg/template/functions.go` — FuncMap: Sprig + custom (`password`, `hostname`, `formatFilesize`, etc.)
- `pkg/template/context.go` — parse `project.yaml` (fallback `project.json`), referenced
  default resolution (topological sort on `[[ ]]` in default values), computed value
  extraction and post-prompt resolution (see [11-computed-values.md](../11-computed-values.md))
- `pkg/template/verbatim.go` — load `.specsverbatim`, glob matching via `fnmatch`
- `pkg/template/template.go` — `Get()`, `Execute()`: filepath.Walk, conditional filenames,
  verbatim copy, binary detection, whitespace-only deletion

---

## Phase 4 — Hooks

**Goal:** Pre/post-use scripts run in the right directory with context as env vars.
**You learn:** `os/exec`, shell subprocess patterns.
**Tests:**
- `Load()`: inline yaml vs `hooks/` directory; error when both are present.
- `Run()`: hook receives correct env vars; non-zero exit aborts with error; command output
  is captured.

Files:

- `pkg/hooks/hooks.go` — `Load()` (inline yaml vs `hooks/` directory, error if both),
  `Run()` (bash -c, inject context as env vars, stop on non-zero exit)

---

## Phase 5 — Git & host utilities

**Goal:** Clone a GitHub repo to disk.
**You learn:** `go-git`.
**Tests:**
- `pkg/host`: unit tests for each source format (github shorthand, with branch, full HTTPS
  URL, local path).
- `pkg/util/git`: integration test that clones a small public repo into a temp directory
  (can be skipped in CI with a build tag).

Files:

- `pkg/util/git/` — `Clone(url, dir)` wrapper around go-git
- `pkg/host/github.go` — parse `github:user/repo[:branch]` and full HTTPS URLs into clone URLs

---

## Phase 6 — Registry commands

**Goal:** All registry-management commands plus shared infrastructure (`osutil`, `validate`,
metadata writing).
**You learn:** Cobra command wiring, `os.MkdirAll`/`os.RemoveAll`, recursive file copy.
**Tests:** Each command tested via `cmd.ExecuteC()` with a temp XDG directory. Test flag
combinations, missing-arg errors, and happy paths.

**Pre-phase setup (already done):** `App` struct (`pkg/cmd/app.go`), `NewApp()`,
`--debug`/`--safe-mode` persistent flags, `slog`-based debug logging, error printing in
`main.go`. See [implementation/phase6.md](implementation/phase6.md) prerequisites.

Files remaining:

- `pkg/util/osutil/` — `CopyDir()` recursive copy
- `pkg/util/validate/` — `Tag()` validator (`AlphanumericExt` — fixes tmrts#61)
- `pkg/cmd/metadata.go` — `writeMetadata()` helper
- `pkg/cmd/init.go` — `specs init [--force]`
- `pkg/cmd/template_list.go` — `specs template list [--dont-prettify]`
- `pkg/cmd/template_save.go` — `specs template save [--force] <path> <tag>`
- `pkg/cmd/template_download.go` — `specs template download [--force] <source> <tag>`
- `pkg/cmd/template_validate.go` — `specs template validate <path>`
- `pkg/cmd/template_rename.go` — `specs template rename <old> <new>`
- `pkg/cmd/template_delete.go` — `specs template delete <tag>...`

---

## Phase 7 — `specs template use`

**Goal:** Interactive template execution with huh prompts, `--values`/`--arg` overrides,
and hooks orchestration.
**You learn:** `huh` — `Input`, `Confirm`, `Select` fields, form composition, pre-filling
answers from `--values`/`--arg`, `--use-defaults` short-circuit.
**Tests:** Command tests using real template directories in `t.TempDir()`; no network access.

Files:

- `pkg/util/values/` — `LoadFile()` (JSON `--values` file), `ParseArg()` (`Key=Value`), `Merge()`
- `pkg/cmd/template_use.go` — `specs template use <tag> <target-dir>`; shared `executeTemplate()` helper
- `pkg/template/template.go` — add public `FuncMap()` method

---

## Phase 8 — `specs use`

**Goal:** One-step command — clone or copy a template, execute it, discard the temp copy.
No registry entry is created.
**You learn:** Composing phases 5–7; temp directory lifecycle.
**Tests:** Local-path tests only (no network); integration tests covered by phase 5.

Files:

- `pkg/cmd/use.go` — `specs use <source> <target-dir>`; reuses `executeTemplate()` from phase 7

---

## CLI command tree

```
specs
├── use <source> <target-dir>               one-step, no registry entry
│     [--values file.yaml]
│     [--arg Key=Value]...
│     [--use-defaults]
│     [--no-hooks]
│
├── template
│   ├── download [--force] <repo> <tag>
│   ├── save     [--force] <path> <tag>
│   ├── use      <tag> <target-dir>
│   │     [--values file.yaml]
│   │     [--arg Key=Value]...
│   │     [--use-defaults]
│   │     [--no-hooks]
│   ├── list     [--dont-prettify]
│   ├── delete   <tag>...
│   ├── validate <path>
│   └── rename   <old> <new>
│
├── init    [--force]
└── version [--dont-prettify]
```

---

## Learning highlights per phase

| Phase | Primary tool/concept |
|-------|----------------------|
| 1 | Cobra — subcommands, flags, `PersistentPreRunE` |
| 2 | lipgloss — styles, colour downsampling, table layout |
| 3 | go-yaml, `text/template` custom delimiters, Sprout registries + FuncMap |
| 4 | `os/exec` subprocess, env injection |
| 5 | go-git clone API |
| 6 | Cobra command wiring, `osutil`, `validate`, registry operations; `App` struct + slog already wired |
| 7 | huh — `Input`, `Confirm`, `Select` fields, form composition, `--values`/`--arg` |
| 8 | Composing phases 5–7; temp directory lifecycle |

---

## Suggested order of work

```
Phase 1 → Phase 2 → Phase 3 → Phase 5 → Phase 6
                                               → Phase 4 (hooks)
                                               → Phase 7 (template use)
                                               → Phase 8 (specs use)
```

Phase 4 (hooks) can be deferred until phase 7 (`specs template use`) since nothing else
depends on it.
