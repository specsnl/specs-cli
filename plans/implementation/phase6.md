# Phase 6 — Registry Commands

## Goal

All registry-management commands (`init`, `list`, `save`, `download`, `validate`, `rename`,
`delete`) plus the shared infrastructure they depend on (`osutil`, `validate`, metadata
writing). No interactive prompts in this phase — that is Phase 7.

## Done criteria

- `specs init` creates the XDG template directory.
- `specs template list` prints a table of registered templates.
- `specs template save` copies a local path into the registry.
- `specs template download` clones a GitHub repo into the registry.
- `specs template validate` validates a template directory.
- `specs template rename` renames a registry entry.
- `specs template delete` removes one or more registry entries.
- `--debug` persistent root flag wires `output.SetDebug(true)`.
- Error printing is wired into `main.go`.
- All tests pass.

---

## Dependencies

No new packages beyond what phases 1–5 added.

---

## File overview

```
main.go                           (updated: print error)
pkg/
├── cmd/
│   ├── root.go                   (updated: --debug flag, PersistentPreRunE)
│   ├── metadata.go               (new: WriteMetadata helper)
│   ├── init.go                   (new)
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

## Shared infrastructure

### `main.go` — wire in error printing

`Execute()` returns the unhandled error from the command. `main.go` must print it before
exiting, otherwise the user sees nothing when a command fails.

```go
func main() {
    if err := cmd.Execute(); err != nil {
        output.Error("%v", err)
        os.Exit(exit.Error)
    }
}
```

---

### `pkg/cmd/root.go` — `--debug` flag and `PersistentPreRunE`

Add a persistent `--debug` flag that enables `output.Debug` calls across all commands.

```go
var debug bool

func init() {
    rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output")
    rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
        output.SetDebug(debug)
        return nil
    }
}
```

---

### `pkg/util/validate/validate.go`

Tag names must be alphanumeric with hyphens and underscores. This fixes the v1 bug
(tmrts#61) where tags containing `-` were rejected.

```go
package validate

import (
    "fmt"
    "regexp"
)

var tagPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Tag returns nil if name is a valid template tag, otherwise a descriptive error.
func Tag(name string) error {
    if name == "" {
        return fmt.Errorf("tag must not be empty")
    }
    if !tagPattern.MatchString(name) {
        return fmt.Errorf("tag %q contains invalid characters (allowed: a-z A-Z 0-9 _ -)", name)
    }
    return nil
}
```

---

### `pkg/util/osutil/osutil.go`

Recursive directory copy. Used by `specs template save` (copy local path into registry)
and by `specs template use` / `specs use` (copy rendered output to the target directory).

```go
package osutil

import (
    "io"
    "io/fs"
    "os"
    "path/filepath"
)

// CopyDir recursively copies the directory tree at src into dst.
// dst is created if it does not exist. Existing files in dst are overwritten.
func CopyDir(src, dst string) error {
    return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        rel, _ := filepath.Rel(src, path)
        target := filepath.Join(dst, rel)

        if d.IsDir() {
            return os.MkdirAll(target, 0755)
        }
        return copyFile(path, target)
    })
}

func copyFile(src, dst string) error {
    if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
        return err
    }
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()
    out, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer out.Close()
    _, err = io.Copy(out, in)
    return err
}
```

---

### `pkg/cmd/metadata.go`

Writes `__metadata.json` after a template is saved or downloaded.

```go
package cmd

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"

    pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
    "github.com/specsnl/specs-cli/pkg/specs"
)

// writeMetadata writes __metadata.json into templateRoot.
func writeMetadata(templateRoot, tag, repository string) error {
    m := pkgtemplate.Metadata{
        Tag:        tag,
        Repository: repository,
        Created:    pkgtemplate.JSONTime{Time: time.Now().UTC()},
    }
    data, err := json.MarshalIndent(m, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(templateRoot, specs.MetadataFile), data, 0644)
}
```

---

## Commands

### `pkg/cmd/init.go` — `specs init`

```
specs init [--force]
```

Creates the XDG template directory. With `--force`, recreates it even if it already exists.

```go
package cmd

import (
    "os"

    "github.com/spf13/cobra"
    "github.com/specsnl/specs-cli/pkg/specs"
    "github.com/specsnl/specs-cli/pkg/util/output"
)

var initForce bool

var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialise the local template registry",
    Args:  cobra.NoArgs,
    RunE: func(cmd *cobra.Command, args []string) error {
        dir := specs.TemplateDir()

        if initForce {
            if err := os.RemoveAll(dir); err != nil {
                return err
            }
        }

        if err := os.MkdirAll(dir, 0755); err != nil {
            return err
        }

        output.Info("registry initialised at %s", dir)
        return nil
    },
}

func init() {
    initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Recreate registry if it already exists")
    rootCmd.AddCommand(initCmd)
}
```

---

### `pkg/cmd/template_list.go` — `specs template list`

```
specs template list [--dont-prettify]
```

Reads all subdirectories from the template registry, loads their `__metadata.json`, and
renders them as a table (or plain text with `--dont-prettify`).

```go
var listDontPrettify bool

var templateListCmd = &cobra.Command{
    Use:   "list",
    Short: "List registered templates",
    Args:  cobra.NoArgs,
    RunE: func(cmd *cobra.Command, args []string) error {
        if !specs.IsRegistryInitialised() {
            return specs.ErrRegistryNotInitialised
        }

        entries, err := os.ReadDir(specs.TemplateDir())
        if err != nil {
            return err
        }

        headers := []string{"Tag", "Repository", "Created"}
        var rows [][]string

        for _, e := range entries {
            if !e.IsDir() {
                continue
            }
            tag := e.Name()
            tmplPath := specs.TemplatePath(tag)

            // Load metadata; gracefully handle missing or malformed files.
            meta, _ := loadMetadataForListing(tmplPath)
            repo, created := "-", "-"
            if meta != nil {
                repo = meta.Repository
                created = meta.Created.String()
            }
            rows = append(rows, []string{tag, repo, created})
        }

        if len(rows) == 0 {
            output.Info("no templates registered — run 'specs template download' or 'specs template save'")
            return nil
        }

        if listDontPrettify {
            for _, row := range rows {
                fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", row[0], row[1], row[2])
            }
            return nil
        }

        fmt.Fprintln(cmd.OutOrStdout(), output.RenderTable(headers, rows))
        return nil
    },
}
```

`loadMetadataForListing` is a thin wrapper around `pkgtemplate.Get` that only reads
metadata — it does not parse `project.yaml`. Implement it directly in this file using
`json.Unmarshal` on `__metadata.json`.

---

### `pkg/cmd/template_save.go` — `specs template save`

```
specs template save [--force] <path> <tag>
```

Registers a local directory as a template in the registry.

```go
var saveForce bool

var templateSaveCmd = &cobra.Command{
    Use:   "save <path> <tag>",
    Short: "Save a local directory as a template",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        srcPath, tag := args[0], args[1]

        if err := validate.Tag(tag); err != nil {
            return err
        }
        if !specs.IsRegistryInitialised() {
            return specs.ErrRegistryNotInitialised
        }

        dest := specs.TemplatePath(tag)
        if _, err := os.Stat(dest); err == nil && !saveForce {
            return specs.ErrTemplateAlreadyExists
        }

        if err := os.RemoveAll(dest); err != nil {
            return err
        }
        if err := osutil.CopyDir(srcPath, dest); err != nil {
            return err
        }
        if err := writeMetadata(dest, tag, srcPath); err != nil {
            return err
        }

        output.Info("template %q saved", tag)
        return nil
    },
}
```

---

### `pkg/cmd/template_download.go` — `specs template download`

```
specs template download [--force] <source> <tag>
```

Clones a remote repository into the registry. `<source>` accepts any format that
`host.Parse()` understands (github shorthand, HTTPS URL, `user/repo:branch`).

```go
var downloadForce bool

var templateDownloadCmd = &cobra.Command{
    Use:   "download <source> <tag>",
    Short: "Download a template from a remote repository",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        rawSource, tag := args[0], args[1]

        if err := validate.Tag(tag); err != nil {
            return err
        }
        if !specs.IsRegistryInitialised() {
            return specs.ErrRegistryNotInitialised
        }

        src, err := host.Parse(rawSource)
        if err != nil {
            return err
        }
        if src.IsLocal() {
            return fmt.Errorf("use 'specs template save' to register a local path")
        }

        dest := specs.TemplatePath(tag)
        if _, err := os.Stat(dest); err == nil && !downloadForce {
            return specs.ErrTemplateAlreadyExists
        }
        if err := os.RemoveAll(dest); err != nil {
            return err
        }

        output.Info("cloning %s…", src.CloneURL)
        if err := git.Clone(src.CloneURL, dest, git.CloneOptions{Branch: src.Branch}); err != nil {
            return err
        }
        if err := writeMetadata(dest, tag, src.CloneURL); err != nil {
            return err
        }

        output.Info("template %q downloaded", tag)
        return nil
    },
}
```

---

### `pkg/cmd/template_validate.go` — `specs template validate`

```
specs template validate <path>
```

Validates a template directory without registering it. Runs a dry execute into a temp dir
using all default values to surface any render errors.

```go
var templateValidateCmd = &cobra.Command{
    Use:   "validate <path>",
    Short: "Validate a template directory",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        templateRoot := args[0]

        // 1. Check template/ subdirectory exists.
        templateDir := filepath.Join(templateRoot, specs.TemplateDirFile)
        if info, err := os.Stat(templateDir); err != nil || !info.IsDir() {
            return specs.ErrTemplateDirMissing
        }

        // 2. Load template (parses project.yaml, resolves referenced defaults + computed).
        tmpl, err := pkgtemplate.Get(templateRoot)
        if err != nil {
            return fmt.Errorf("invalid template: %w", err)
        }

        // 3. Dry execute into a temp directory — discard output.
        tmp, err := os.MkdirTemp("", "specs-validate-*")
        if err != nil {
            return err
        }
        defer os.RemoveAll(tmp)

        if err := tmpl.Execute(tmp); err != nil {
            return fmt.Errorf("template render error: %w", err)
        }

        output.Info("template is valid")
        return nil
    },
}
```

---

### `pkg/cmd/template_rename.go` — `specs template rename`

```
specs template rename <old> <new>
```

Renames a registry entry by moving its directory.

```go
var templateRenameCmd = &cobra.Command{
    Use:   "rename <old-tag> <new-tag>",
    Short: "Rename a registered template",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        oldTag, newTag := args[0], args[1]

        if err := validate.Tag(newTag); err != nil {
            return err
        }
        if !specs.IsRegistryInitialised() {
            return specs.ErrRegistryNotInitialised
        }

        src := specs.TemplatePath(oldTag)
        if _, err := os.Stat(src); os.IsNotExist(err) {
            return fmt.Errorf("%w: %s", specs.ErrTemplateNotFound, oldTag)
        }

        dst := specs.TemplatePath(newTag)
        if _, err := os.Stat(dst); err == nil {
            return fmt.Errorf("tag %q already exists — delete it first", newTag)
        }

        if err := os.Rename(src, dst); err != nil {
            return err
        }

        output.Info("template %q renamed to %q", oldTag, newTag)
        return nil
    },
}
```

---

### `pkg/cmd/template_delete.go` — `specs template delete`

```
specs template delete <tag> [<tag>...]
```

Removes one or more registry entries.

```go
var templateDeleteCmd = &cobra.Command{
    Use:   "delete <tag> [<tag>...]",
    Short: "Delete one or more registered templates",
    Args:  cobra.MinimumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        if !specs.IsRegistryInitialised() {
            return specs.ErrRegistryNotInitialised
        }

        for _, tag := range args {
            if err := validate.Tag(tag); err != nil {
                return err
            }
            path := specs.TemplatePath(tag)
            if _, err := os.Stat(path); os.IsNotExist(err) {
                return fmt.Errorf("%w: %s", specs.ErrTemplateNotFound, tag)
            }
            if err := os.RemoveAll(path); err != nil {
                return err
            }
            output.Info("template %q deleted", tag)
        }
        return nil
    },
}
```

---

## Tests

All command tests use a helper that wires up a temp XDG directory and executes the command
via `rootCmd.ExecuteC()` with captured output.

### Test helper pattern

```go
// executeCmd sets up a temp XDG environment, sets args, and captures stdout.
func executeCmd(t *testing.T, args ...string) (string, error) {
    t.Helper()
    tmp := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tmp)
    xdg.Reload()
    t.Cleanup(func() { xdg.Reload() })

    buf := new(bytes.Buffer)
    rootCmd.SetOut(buf)
    rootCmd.SetErr(buf)
    rootCmd.SetArgs(args)
    _, err := rootCmd.ExecuteC()
    return buf.String(), err
}
```

### `pkg/util/validate/validate_test.go`

| Test | Input | Expected |
|---|---|---|
| `TestTag_Valid` | `my-template`, `template_1`, `UPPER` | nil error |
| `TestTag_Empty` | `""` | error |
| `TestTag_InvalidChars` | `"foo bar"`, `"foo/bar"`, `"foo.bar"` | error |

### `pkg/util/osutil/osutil_test.go`

| Test | Setup | Expected |
|---|---|---|
| `TestCopyDir_PreservesStructure` | nested dirs and files in src | same structure in dst |
| `TestCopyDir_PreservesContent` | file with known content | dst file has same content |
| `TestCopyDir_OverwritesExisting` | dst file already exists | overwritten with src content |

### `pkg/cmd/` command tests

| Test | Args | Expected |
|---|---|---|
| `TestInit_CreatesDir` | `init` | registry dir exists |
| `TestInit_Force` | `init --force` (dir exists) | dir recreated |
| `TestInit_AlreadyExists` | `init` twice | second call succeeds (idempotent) |
| `TestList_NotInitialised` | `template list` (no init) | returns `ErrRegistryNotInitialised` |
| `TestList_Empty` | `template list` (after init, no templates) | info message, no error |
| `TestList_ShowsTemplate` | `template list` after saving | output contains tag name |
| `TestList_DontPrettify` | `template list --dont-prettify` | tab-separated output |
| `TestSave_Success` | `template save <path> my-tag` | registry entry created |
| `TestSave_AlreadyExists` | save same tag twice | error on second |
| `TestSave_Force` | `template save --force <path> my-tag` twice | succeeds both times |
| `TestSave_InvalidTag` | `template save <path> "bad tag"` | error |
| `TestDownload_LocalSourceRejected` | `template download ./path my-tag` | error (use save) |
| `TestValidate_ValidTemplate` | valid template dir | exits 0, prints "valid" |
| `TestValidate_MissingTemplateDir` | root without `template/` | error |
| `TestRename_Success` | rename existing tag | new tag exists, old gone |
| `TestRename_NotFound` | rename non-existent tag | error |
| `TestDelete_Success` | delete existing tag | tag directory removed |
| `TestDelete_NotFound` | delete non-existent tag | error |
| `TestDelete_MultipleArgs` | delete two tags | both removed |

---

## Key notes

- **Command registration:** each command file has an `init()` that calls
  `templateCmd.AddCommand(...)` or `rootCmd.AddCommand(...)`. No changes to `root.go` or
  `template.go` are needed beyond adding `--debug` and `PersistentPreRunE`.
- **`template list` metadata loading:** read `__metadata.json` directly with `json.Unmarshal`
  in the command file. Do not call `pkgtemplate.Get()` — that parses `project.yaml` too,
  which is unnecessary and slow when listing.
- **`os.Rename` for rename:** atomic on the same filesystem, which is always the case here
  (both src and dst are inside the XDG template directory). No need for copy+delete.
- **`cobra.ExactArgs` vs manual validation:** prefer `cobra.ExactArgs(n)` and
  `cobra.MinimumNArgs(n)` over manual `len(args)` checks — Cobra formats the error message
  consistently and `SilenceUsage` ensures it is not shown on runtime errors.
- **`template download` accepts any `host.Parse()` format:** the flag/argument name in the
  help text is `<source>`, not `<github-repo>`, to reflect that HTTPS URLs are equally valid.
- **`--dont-prettify` on `template list`:** when set, output is tab-separated with no
  borders — suitable for shell scripting (`specs template list --dont-prettify | awk '{print $1}'`).
