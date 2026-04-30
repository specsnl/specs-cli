# Phase 8 — `specs use`

## Goal

The one-step command. Parse a source string (GitHub shorthand, HTTPS URL, or local path),
obtain the template (clone or copy), execute it into the target directory, then discard the
temporary copy. No registry entry is created.

## Done criteria

- `specs use <source> <target-dir>` works for GitHub shorthands, HTTPS URLs, and local paths.
- Remote sources are cloned with `git.Clone()` into a temp directory.
- Local sources are copied with `osutil.CopyDir()` into a temp directory.
- The temp directory is always cleaned up, even if execution fails.
- All flags from `specs template use` are supported: `--values`, `--arg`, `--use-defaults`,
  `--no-hooks`.
- All tests pass.

---

## Dependencies

No new packages. All dependencies are already present from phases 3–7.

---

## File overview

```
pkg/
└── cmd/
    └── use.go    (new)
```

---

## Files

### `pkg/cmd/use.go`

> **Implementation note:** The codebase uses constructor functions (e.g. `newTemplateCmd(app)`)
> rather than package-level vars + `init()`. `use.go` follows this pattern. `opts executeOpts`
> is a local variable inside `newUseCmd`; flag bindings write into its fields directly.

```go
package cmd

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"

    "github.com/specsnl/specs-cli/pkg/host"
    pkggit "github.com/specsnl/specs-cli/pkg/util/git"
    "github.com/specsnl/specs-cli/pkg/util/osutil"
    "github.com/specsnl/specs-cli/pkg/util/output"
)

func newUseCmd(app *App) *cobra.Command {
    var opts executeOpts

    cmd := &cobra.Command{
        Use:   "use <source> <target-dir>",
        Short: "Fetch and execute a template in one step (no registry entry created)",
        Long: `...`,
        Args: cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runUse(app, args[0], args[1], opts)
        },
    }

    cmd.Flags().StringVar(&opts.valuesFile, "values", "", "JSON file of pre-filled values")
    cmd.Flags().StringArrayVar(&opts.argPairs, "arg", nil, "Key=Value pair (repeatable)")
    cmd.Flags().BoolVar(&opts.useDefaults, "use-defaults", false, "Skip prompts; use schema defaults")
    cmd.Flags().BoolVar(&opts.noHooks, "no-hooks", false, "Skip pre/post-use hooks")

    return cmd
}

func runUse(app *App, rawSource, targetDir string, opts executeOpts) error {
    src, err := host.Parse(rawSource)
    if err != nil {
        return err
    }

    // Parent temp dir — always cleaned up, even on error.
    tmp, err := os.MkdirTemp("", "specs-use-src-*")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmp)

    var templateRoot string

    if src.IsLocal() {
        if err := osutil.CopyDir(src.LocalPath, tmp); err != nil {
            return fmt.Errorf("copying local template: %w", err)
        }
        templateRoot = tmp
    } else {
        // go-git requires the destination to not exist; use a subdirectory.
        cloneDir := filepath.Join(tmp, "repo")
        output.Info("cloning %s…", src.CloneURL)
        if err := pkggit.Clone(src.CloneURL, cloneDir, pkggit.CloneOptions{Branch: src.Branch}); err != nil {
            return err
        }
        templateRoot = cloneDir
    }

    return app.executeTemplate(templateRoot, targetDir, opts)
}
```

Register in `pkg/cmd/root.go` by adding:

```go
cmd.AddCommand(newUseCmd(app))
```

---

## Tests

### `pkg/cmd/use_test.go`

> **Implementation note:** The `executeCmd` test helper signature is `executeCmd(args ...string)`
> — it does **not** take `*testing.T` as its first argument.

```go
// buildMinimalTemplate creates a valid template root in dir with the given project.yaml
// content and a single template file.
func buildMinimalTemplate(t *testing.T, dir, yamlContent, filename, fileContent string) {
    t.Helper()
    os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(yamlContent), 0644)
    tplDir := filepath.Join(dir, specs.TemplateDirFile)
    os.MkdirAll(tplDir, 0755)
    os.WriteFile(filepath.Join(tplDir, filename), []byte(fileContent), 0644)
}
```

| Test | Setup | Expected |
|---|---|---|
| `TestUse_LocalPath` | local template dir, `file:./path` source | target has rendered output |
| `TestUse_RelativePath` | local template, absolute path source | target has rendered output |
| `TestUse_UseDefaults` | `--use-defaults` flag | rendered with schema defaults, no prompt |
| `TestUse_ArgOverride` | `--arg Name=test` | rendered content contains "test" |
| `TestUse_ValuesFile` | `--values` JSON file | rendered content from file values |
| `TestUse_InvalidSource` | `"not-a-valid-source"` | returns parse error |
| `TestUse_TempDirCleanedUp` | any successful run | temp clone dir no longer exists after |
| `TestUse_TempDirCleanedOnError` | missing `template/` subdir (parse fails) | temp dir still cleaned up |
| `TestUse_NoRegistryEntry` | successful run | registry dir untouched (no entry created) |

### Verifying no registry entry is created

```go
func TestUse_NoRegistryEntry(t *testing.T) {
    // withTempRegistry sets XDG_CONFIG_HOME to a temp dir and reloads xdg.
    // No separate init command is needed — os.ReadDir on a missing dir returns nil.
    withTempRegistry(t)

    srcDir := t.TempDir()
    buildMinimalTemplate(t, srcDir, "Name: world\n", "hello.txt", "Hello [[.Name]]")
    targetDir := t.TempDir()
    _, err := executeCmd("use", "--use-defaults", "file:"+srcDir, targetDir)
    if err != nil {
        t.Fatalf("use: %v", err)
    }

    // TemplateDir may not exist at all, or may exist but be empty — either is correct.
    entries, _ := os.ReadDir(specs.TemplateDir())
    if len(entries) != 0 {
        t.Errorf("registry should be empty after specs use, got %d entries", len(entries))
    }
}
```

---

## Key notes

- **Constructor pattern:** `newUseCmd(app *App) *cobra.Command` — not a global `var useCmd`.
  Registered in `root.go` alongside the other root-level commands. `opts executeOpts` is a
  local variable; flag bindings write directly into its fields via `&opts.valuesFile` etc.
- **`output.Debug` does not exist.** The `output` package exposes only `Info`, `Warn`, and
  `Error`. The local-copy debug log call in earlier drafts was removed. Use `output.Info` for
  user-facing messages and `app.Logger.Debug` (slog) if debug-level logging is ever needed.
- **`runUse` signature:** `runUse(app *App, rawSource, targetDir string, opts executeOpts) error`
  — extracted so flags are bound via the `newUseCmd` closure and the function remains testable.
- **No `init` command.** Phase 6 lists `specs init [--force]` but it has not been implemented.
  Tests that need an empty registry use `withTempRegistry(t)` directly and do not call a CLI
  `init` command.
- **`osutil.CopyDir` into a fresh temp dir for local sources.** Even for local paths the
  template is copied into a temp directory before execution. This ensures the template engine
  always sees a stable, isolated tree — hooks running in `pre-use` cannot accidentally modify
  the original source.
- **Temp dir naming prefix:** use `"specs-use-src-*"` for the source temp dir (cloned or
  copied template) and note that `executeTemplate` also creates its own `"specs-use-*"` temp
  dir for the render output. Two temp dirs are expected: one for the source, one for the
  rendered output before it is copied to `targetDir`.
- **`defer os.RemoveAll(tmp)` on the source temp dir is in `runUse`.** The render output
  temp dir is managed inside `executeTemplate` (Phase 7). Both are always cleaned up.
- **`git clone` writes into a non-existent directory:** `gogit.PlainClone` requires that
  the destination directory does not already exist. `os.MkdirTemp` creates the directory, so
  use a subdirectory:

  ```go
  tmp, _ := os.MkdirTemp("", "specs-use-src-*")
  cloneDir := filepath.Join(tmp, "repo")
  // cloneDir does not exist yet — safe to pass to git.Clone
  pkggit.Clone(src.CloneURL, cloneDir, ...)
  // then use cloneDir as templateRoot
  ```

  This avoids the remove-then-recreate race and is the recommended approach.
- **Local path resolution:** `host.Parse("./my-template")` returns a `LocalPath` of
  `"./my-template"`. This is relative to the process working directory (where the user ran
  `specs`), not the binary location. `osutil.CopyDir` with a relative path works correctly
  because `filepath.WalkDir` resolves it against the working directory.
- **`specs use` is a root-level command, not under `template`.** It is registered on
  `rootCmd` directly. The shorter invocation (`specs use`) vs `specs template use` signals
  to users that this is the primary workflow.
