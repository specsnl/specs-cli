# Boilr — CLI Commands

## Root Command (`pkg/cmd/root.go`)

The root Cobra command registers all sub-commands and sets persistent flags inherited by children.

---

## `boilr init`

**File:** `pkg/cmd/init.go`

Ensures the local template registry exists.

```
boilr init [--force]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--force` | `-f` | `false` | Recreate registry even if it already exists |

**Flow:**
1. If `--force`, delete existing registry directory.
2. `osutil.CreateDirs(TemplateDirPath)`.
3. Print success.

---

## `boilr template download`

**File:** `pkg/cmd/download.go`

Downloads a template from GitHub and registers it locally.

```
boilr template download <github-repo> <name>
                         e.g. tmrts/boilr-license  license
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--force` | `-f` | `false` | Overwrite if name already exists |
| `--log-level` | `-l` | `error` | Logging verbosity |

**Flow:**
1. Validate exactly 2 args; validate `name` is alphanumeric-ext.
2. Check registry is initialised.
3. If name exists and `--force` not set → error.
4. `host.URL(repo)` → normalise to `https://github.com/user/repo`.
5. `git.Clone(registryPath/name, CloneOptions{URL})`.
6. `serializeMetadata(name, repo, now)` → `__metadata.json`.

---

## `boilr template save`

**File:** `pkg/cmd/save.go`

Registers a local directory as a template.

```
boilr template save <template-path> <name>
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--force` | `-f` | `false` | Overwrite existing name |

**Flow:**
1. Validate args; validate `name`.
2. Check registry initialised.
3. If name exists and not `--force` → error.
4. `exec.Cmd("cp", "-r", srcPath, registryPath/name)`.
5. `serializeMetadata(name, srcPath, now)`.

---

## `boilr template use`

**File:** `pkg/cmd/use.go`

Executes a registered template and writes output to a target directory.

```
boilr template use <name> <target-dir>
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--use-defaults` | `-f` | `false` | Skip prompts; use defaults from `project.json` |
| `--log-level` | `-l` | `error` | Logging verbosity |

**Flow:**
1. Validate args; check registry; check name exists.
2. `template.Get(registryPath/name)` → loads `project.json` + `__metadata.json`.
3. If `--use-defaults` → `tmpl.UseDefaultValues()`.
4. `os.MkdirTemp(...)` → temporary staging directory.
5. `tmpl.Execute(tmpDir)` → renders template into staging dir.
6. `osutil.CopyRecursively(tmpDir, targetDir)`.
7. Cleanup tmp; print success.

---

## `boilr template list`

**File:** `pkg/cmd/list.go`
**Aliases:** `ls`

Displays all registered templates.

```
boilr template list [--dont-prettify]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dont-prettify` | `false` | Raw output instead of coloured table |

**Output columns:** Name · Repository · Age

**Also returns** a `map[string]bool` used by other commands for quick existence checks.

---

## `boilr template delete`

**File:** `pkg/cmd/delete.go`
**Aliases:** `remove`, `rm`, `del`

Removes one or more registered templates.

```
boilr template delete <name> [<name>...]
```

**Flow:**
1. Validate at least 1 arg; each name alphanumeric-ext.
2. For each name: `os.RemoveAll(registryPath/name)`.

---

## `boilr template validate`

**File:** `pkg/cmd/validate.go`

Validates a template directory without registering it.

```
boilr template validate <template-path>
```

**Flow:**
1. Check `template/` subdirectory exists inside path.
2. Do a dry-run execution with default values.
3. Exit success or print validation errors.

---

## `boilr template rename` *(hidden)*

**File:** `pkg/cmd/rename.go`
**Aliases:** `mv`

```
boilr template rename <old-name> <new-name>
```

Renames a template entry in the registry by moving its directory.

---

## `boilr version`

**File:** `pkg/cmd/version.go`

```
boilr version [--dont-prettify]
```

Displays `Version`, `BuildDate` (UTC), and `Commit` hash embedded at build time via ldflags.

---

## `boilr configure-bash-completion` *(hidden)*

**File:** `pkg/cmd/bash_completion.go`

Generates a bash completion script and appends a `source` line to `~/.bashrc`.

---

## Shared Command Infrastructure

### `pkg/cmd/flags.go`

```go
GetBoolFlag(cmd, name)    string
GetStringFlag(cmd, name)  string
```

Typed accessors over `cmd.Flags()`.

### `pkg/cmd/must_validate.go`

```go
MustValidateArgs(cmd, args, expected, validators...)
MustValidateVarArgs(cmd, args, min, validators...)
MustValidateTemplate(path)
MustValidateTemplateDir()
```

All `Must*` functions call `exit.Fatal(err)` on failure — no error propagation needed in command bodies.

### `pkg/cmd/metadata.go`

```go
serializeMetadata(name, repository string, t time.Time) error
```

Writes `__metadata.json` alongside a newly saved or downloaded template.
