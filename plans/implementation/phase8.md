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

```go
package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"

    "github.com/specsnl/specs-cli/pkg/host"
    pkggit "github.com/specsnl/specs-cli/pkg/util/git"
    "github.com/specsnl/specs-cli/pkg/util/osutil"
    "github.com/specsnl/specs-cli/pkg/util/output"
)

var (
    useValuesFile string
    useArgs       []string
    useDefaults   bool
    useNoHooks    bool
)

var useCmd = &cobra.Command{
    Use:   "use <source> <target-dir>",
    Short: "Fetch and execute a template in one step (no registry entry created)",
    Long: `Download a template from a remote repository (or copy a local path) and
execute it directly into <target-dir>. No entry is added to the local registry.

Source formats:
  github:user/repo            GitHub shorthand (default branch)
  github:user/repo:branch     GitHub shorthand with specific branch
  https://github.com/user/repo  Full HTTPS URL
  ./path  ../path  /path      Local path
  file:./path                 Local path with explicit prefix`,
    Args: cobra.ExactArgs(2),
    RunE: runUse,
}

func init() {
    useCmd.Flags().StringVar(&useValuesFile, "values", "", "JSON file of pre-filled values")
    useCmd.Flags().StringArrayVar(&useArgs, "arg", nil, "Key=Value pair (repeatable)")
    useCmd.Flags().BoolVar(&useDefaults, "use-defaults", false, "Skip prompts; use schema defaults")
    useCmd.Flags().BoolVar(&useNoHooks, "no-hooks", false, "Skip pre/post-use hooks")
    rootCmd.AddCommand(useCmd)
}

func runUse(cmd *cobra.Command, args []string) error {
    rawSource, targetDir := args[0], args[1]

    src, err := host.Parse(rawSource)
    if err != nil {
        return err
    }

    // Obtain the template into a temp directory.
    tmp, err := os.MkdirTemp("", "specs-use-src-*")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmp)

    if src.IsLocal() {
        output.Debug("copying local template from %s", src.LocalPath)
        if err := osutil.CopyDir(src.LocalPath, tmp); err != nil {
            return fmt.Errorf("copying local template: %w", err)
        }
    } else {
        output.Info("cloning %s…", src.CloneURL)
        if err := pkggit.Clone(src.CloneURL, tmp, pkggit.CloneOptions{Branch: src.Branch}); err != nil {
            return err
        }
    }

    // Reuse the shared executeTemplate function from phase 7.
    return executeTemplate(tmp, targetDir, executeOpts{
        valuesFile:  useValuesFile,
        argPairs:    useArgs,
        useDefaults: useDefaults,
        noHooks:     useNoHooks,
    })
}
```

---

## Tests

### `pkg/cmd/use_test.go`

All tests use local template directories in `t.TempDir()` — no network access. Remote
clone paths are tested at the integration level (see phase 5 integration tests).

```go
// buildMinimalTemplate creates a valid template root in dir with the given project.yaml
// content and a single template file.
func buildMinimalTemplate(t *testing.T, dir, yaml, filename, content string) {
    t.Helper()
    os.WriteFile(filepath.Join(dir, "project.yaml"), []byte(yaml), 0644)
    tplDir := filepath.Join(dir, "template")
    os.MkdirAll(tplDir, 0755)
    os.WriteFile(filepath.Join(tplDir, filename), []byte(content), 0644)
}
```

| Test | Setup | Expected |
|---|---|---|
| `TestUse_LocalPath` | local template dir, `file:./path` source | target has rendered output |
| `TestUse_RelativePath` | local template, `./path` source | target has rendered output |
| `TestUse_UseDefaults` | `--use-defaults` flag | rendered with schema defaults, no prompt |
| `TestUse_ArgOverride` | `--arg Name=test` | rendered content contains "test" |
| `TestUse_ValuesFile` | `--values` JSON file | rendered content from file values |
| `TestUse_InvalidSource` | `"not-a-valid-source"` | returns parse error |
| `TestUse_TempDirCleanedUp` | any successful run | temp clone dir no longer exists after |
| `TestUse_TempDirCleanedOnError` | invalid project.yaml (parse fails) | temp dir still cleaned up |
| `TestUse_NoRegistryEntry` | successful run | registry dir untouched (no entry created) |

### Verifying no registry entry is created

```go
func TestUse_NoRegistryEntry(t *testing.T) {
    // Set up XDG + init registry.
    tmp := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tmp)
    xdg.Reload()
    t.Cleanup(func() { xdg.Reload() })
    executeCmd(t, "init")

    // Build a local template and run specs use.
    srcDir := t.TempDir()
    buildMinimalTemplate(t, srcDir, "Name: world\n", "hello.txt", "Hello [[.Name]]")
    targetDir := t.TempDir()
    _, err := executeCmd(t, "use", "--use-defaults", "file:"+srcDir, targetDir)
    if err != nil {
        t.Fatalf("use: %v", err)
    }

    // Registry template dir should still be empty.
    entries, _ := os.ReadDir(specs.TemplateDir())
    if len(entries) != 0 {
        t.Errorf("registry should be empty after specs use, got %d entries", len(entries))
    }
}
```

---

## Key notes

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
  it must be removed before cloning. Either remove it immediately after creation and pass the
  path to Clone, or pass a subdirectory:

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
