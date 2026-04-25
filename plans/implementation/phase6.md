# Phase 6 — Registry Commands

## Goal

All registry-management commands (`list`, `save`, `download`, `validate`, `rename`,
`delete`) plus the shared infrastructure they depend on (`osutil`, `validate`, metadata
writing). No interactive prompts in this phase — that is Phase 7.

The registry is created automatically on first use — no explicit `init` step required.
A hidden `reset-registry` command is available for wiping and recreating the registry.

## Prerequisites (already done before this phase)

The following were implemented as a pre-phase-6 setup step and do not need to be repeated:

- `pkg/cmd/app.go` — `App` struct with `*slog.Logger`, `*slog.LevelVar`, `SafeMode bool`; `NewApp()`, `SetDebug()`, `templateConfig()`
- `main.go` — creates `App`, passes it to `Execute(app)`, prints errors via `output.Error`
- `pkg/cmd/root.go` — rewritten as `newRootCmd(app *App)`; `--debug` and `--safe-mode` persistent flags; `PersistentPreRunE` configures the App
- `pkg/cmd/template.go` / `version.go` — rewritten as constructor functions (`newTemplateCmd()`, `newVersionCmd()`)
- `pkg/util/output/log.go` — `Debug`/`SetDebug`/`debugEnabled` removed; debug logging is now `slog.Debug(...)` throughout
- `pkg/template/template.go` — `output.Debug(...)` replaced with `slog.Debug(...)`

## Done criteria

- All template commands auto-create the registry on first use.
- `specs template list` prints a table of registered templates.
- `specs template save` copies a local path into the registry.
- `specs template download` clones a remote repo into the registry.
- `specs template validate` validates a template directory.
- `specs template rename` renames a registry entry.
- `specs template delete` removes one or more registry entries.
- `reset-registry` (hidden) wipes and recreates the registry.
- All tests pass.

---

## Dependencies

No new packages beyond what phases 1–5 added.

---

## File overview

```
pkg/
├── cmd/
│   ├── app.go                    (done — App struct, NewApp, SetDebug, templateConfig)
│   ├── root.go                   (done — newRootCmd(app), --debug/--safe-mode flags)
│   ├── template.go               (done — newTemplateCmd(app))
│   ├── version.go                (done — newVersionCmd())
│   ├── metadata.go               (new: writeMetadata helper)
│   ├── reset_registry.go         (new: hidden reset-registry command)
│   ├── template_list.go          (new)
│   ├── template_save.go          (new)
│   ├── template_download.go      (new)
│   ├── template_validate.go      (new)
│   ├── template_rename.go        (new)
│   └── template_delete.go        (new)
└── util/
    ├── osutil/
    │   └── osutil.go             (new)
    └── validate/
        └── validate.go           (new)
```

---

## Registry auto-creation

`specs.EnsureRegistry()` in `pkg/specs/configuration.go` calls `os.MkdirAll` on the
template directory. Every command that needs the registry calls this instead of checking
`IsRegistryInitialised()`. There is no sentinel error for an uninitialised registry.

```go
// EnsureRegistry creates the template registry directory if it does not already exist.
func EnsureRegistry() error {
    return os.MkdirAll(TemplateDir(), 0755)
}
```

---

## Shared infrastructure

### `pkg/util/validate/validate.go`

Template names must be alphanumeric with hyphens and underscores.

```go
var namePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func Name(name string) error {
    if name == "" {
        return fmt.Errorf("name must not be empty")
    }
    if !namePattern.MatchString(name) {
        return fmt.Errorf("name %q contains invalid characters (allowed: a-z A-Z 0-9 _ -)", name)
    }
    return nil
}
```

### `pkg/util/osutil/osutil.go`

Recursive directory copy. Used by `specs template save` and `specs template use` / `specs use`.

### `pkg/cmd/metadata.go`

Writes `__metadata.json` after a template is saved or downloaded.

---

## Commands

### `reset-registry` (hidden)

Wipes and recreates the local template registry. Not shown in `specs --help`.

```
specs reset-registry
```

### `specs template list [--dont-prettify]`

Reads all subdirectories from the registry, loads their `__metadata.json`, renders a
table. With `--dont-prettify`, outputs tab-separated plain text for scripting.

### `specs template save [--force] <path> <name>`

Copies a local directory into the registry under the given name.

### `specs template download [--force] <source> <name>`

Clones a remote repository into the registry. `<source>` accepts any format that
`host.Parse()` understands (github shorthand, HTTPS URL, SSH URL).

### `specs template validate <path>`

Validates a template directory without registering it. Dry-executes into a temp dir
using all default values.

### `specs template rename <old-name> <new-name>`

Renames a registry entry using `os.Rename` (atomic on the same filesystem).

### `specs template delete <name>...`

Removes one or more registry entries.

---

## Key notes

- **No `specs init`.** The registry is auto-created. Users never need to think about it.
- **`reset-registry` is hidden** — available to power users but not visible in help output.
- **`os.Rename` for rename:** atomic on the same filesystem (both src and dst are inside
  the XDG template directory).
- **`template list` metadata loading:** reads `__metadata.json` directly with
  `json.Unmarshal`. Does not call `pkgtemplate.Get()` — that parses `project.yaml` too,
  which is unnecessary when listing.
- **`template download` accepts any `host.Parse()` format:** the argument is named
  `<source>` to reflect that HTTPS and SSH URLs are equally valid.
