# Boilr — Current Architecture

## Overview

Boilr is a CLI tool written in Go that generates boilerplate project structures from
templates. Templates are Git repositories with a prescribed layout; boilr downloads them,
stores them in a local registry, and uses Go's `text/template` engine (extended with Sprig
and custom functions) to render them into a target directory.

---

## Repository Layout

```
boilr/
├── boilr.go                  # main() entry point
├── go.mod / go.sum           # module definition & lockfile
├── Taskfile.dist.yml         # developer task runner
├── .goreleaser.yml           # release pipeline config
├── assets/                   # logo, usage gif
├── wiki/                     # user-facing documentation
└── pkg/
    ├── boilr/                # global config & sentinel errors
    ├── cmd/                  # cobra commands (one file per command)
    ├── host/                 # GitHub URL helpers
    ├── prompt/               # interactive stdin prompts
    ├── template/             # template loading & execution engine
    └── util/                 # small utility sub-packages
        ├── exec/
        ├── exit/
        ├── git/
        ├── osutil/
        ├── stringutil/
        ├── tabular/
        ├── tlog/
        └── validate/
```

---

## Startup Flow

```
boilr.go  main()
  ├─ osutil.DirExists(TemplateDirPath)   // ~/.config/boilr/templates
  ├─ osutil.CreateDirs(TemplateDirPath)  // create if missing
  └─ cmd.Run()                           // hand off to Cobra
```

---

## CLI Command Tree

```
boilr
├── init [--force]
├── template
│   ├── download [--force] [--log-level] <repo> <name>
│   ├── save     [--force] <path> <name>
│   ├── use      [--use-defaults] [--log-level] <name> <dir>
│   ├── list     [--dont-prettify]
│   ├── delete   <name>...
│   ├── validate <path>
│   └── rename   <old-name> <new-name>       (hidden)
├── version      [--dont-prettify]
└── configure-bash-completion              (hidden)
```

See [02-cli-commands.md](./02-cli-commands.md) for per-command detail.

---

## Template Registry

All templates are stored on disk under `~/.config/boilr/`:

```
~/.config/boilr/
├── config.json           # optional user overrides
├── completion.bash       # bash completion script
└── templates/
    └── <name>/
        ├── project.json          # context schema & defaults
        ├── __metadata.json       # name, repo URL, creation time
        └── template/             # rendered file tree lives here
            ├── {{Name}}.go
            ├── README.md
            └── ...
```

`project.json` defines the variables that boilr will prompt for.
`__metadata.json` is written automatically when a template is downloaded or saved.

---

## Data Flow — `boilr template use`

```
boilr template use <name> <target-dir>
        │
        ▼
  validate args & check registry
        │
        ▼
  template.Get(<registry-path>)
  ├─ read project.json  →  Context map
  ├─ read __metadata.json
  └─ build dirTemplate{Path, Context, FuncMap}
        │
        ▼
  BindPrompts()  (or BindDefaults() with --use-defaults)
  └─ for each key in Context:
       prompt.New(key, defaultValue) → user answer
        │
        ▼
  Execute(tmpDir)
  └─ filepath.Walk(template/)
       for each entry:
         ├─ render filename  (Go text/template)
         ├─ if directory     → mkdir
         ├─ if binary file   → copy as-is
         └─ if text file     → render content → write
        │
        ▼
  osutil.CopyRecursively(tmpDir, targetDir)
        │
        ▼
  tlog.Success(...)
```

---

## Key Packages Summary

| Package | Role |
|---------|------|
| `pkg/boilr` | Global constants (`AppName`, config paths, `IgnoreCopyFiles`) |
| `pkg/cmd` | One Cobra command per file; shared helpers in `flags.go`, `must_validate.go`, `metadata.go` |
| `pkg/template` | `Get()` factory, `Execute()` walker, `BindPrompts()`, `FuncMap` |
| `pkg/prompt` | `strPrompt`, `boolPrompt`, `multipleChoicePrompt` |
| `pkg/host` | `URL()` / `ZipURL()` — normalise GitHub identifiers |
| `pkg/util/git` | Thin wrapper around `go-git` `Clone()` |
| `pkg/util/osutil` | `FileExists`, `DirExists`, `CreateDirs`, `CopyRecursively` |
| `pkg/util/exec` | `Cmd()` — run a subprocess, capture stdout/stderr |
| `pkg/util/exit` | Typed exit codes (`OK`, `Error`, `Fatal`, `GoodEnough`) |
| `pkg/util/tlog` | Levelled, coloured terminal logger with Unicode symbols |
| `pkg/util/tabular` | Pretty-print tables with `tablewriter` |
| `pkg/util/validate` | Regex patterns + named argument validators |
| `pkg/util/stringutil` | `io.ReadWriter` wrapper over a plain string |

---

## External Dependencies

| Dependency | Purpose |
|-----------|---------|
| `github.com/spf13/cobra v1.9.1` | CLI framework |
| `github.com/Masterminds/sprig v2.22.0` | Extra template functions |
| `github.com/go-git/go-git/v5 v5.16.2` | Pure-Go Git clone |
| `github.com/fatih/color v1.18.0` | Coloured terminal output |
| `github.com/olekukonko/tablewriter v0.0.5` | Table rendering |
| `github.com/ryanuber/go-glob v1.0.0` | Glob matching |
| `github.com/sethvargo/go-password v0.3.1` | Secure password generation |
| `github.com/docker/go-units v0.5.0` | Human-readable file sizes |

---

## Architectural Patterns in Use

| Pattern | Where |
|---------|-------|
| Command | Every Cobra sub-command is a self-contained unit |
| Factory | `template.Get()` returns an `Interface` implementation |
| Strategy | Prompt types (`strPrompt`, `boolPrompt`, `multipleChoicePrompt`) |
| Interface segregation | `template.Interface`, `prompt.Interface` |
| Decorator | Sprig + custom `FuncMap` extend Go's `text/template` |

---

## CI / Release

```
.github/workflows/
├── build.yml      # go build on push/PR
├── lint.yml       # golangci-lint
├── release.yml    # goreleaser on tag push
└── dependabot.yml # auto dependency PRs
```

Release artefacts are produced by [GoReleaser](https://goreleaser.com) (`.goreleaser.yml`),
which cross-compiles binaries and publishes them to GitHub Releases.
