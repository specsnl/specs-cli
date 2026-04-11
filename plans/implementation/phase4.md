# Phase 4 — Hooks

## Goal

Load hook definitions from `project.yaml` (inline) or a `hooks/` directory, and execute
pre/post-use scripts in the correct working directory with the resolved context injected as
environment variables.

## Done criteria

- `hooks.Load()` reads inline hooks from `project.yaml` and from a `hooks/` directory.
- `hooks.Load()` returns an error when both sources are present.
- `hooks.Run()` executes commands via `bash -c`, injecting context keys as env vars.
- Hook commands may contain `[[ ]]` template expressions resolved against the final context.
- A non-zero exit from any hook command returns an error and stops further execution.
- All tests pass.

---

## Dependencies

No new packages. Hooks use `os/exec` (stdlib) and the `text/template` engine already added
in Phase 3.

---

## File overview

```
pkg/
└── hooks/
    └── hooks.go
```

---

## Hook definition formats

### Inline (`project.yaml`)

```yaml
hooks:
  pre-use:
    - echo "Starting [[.ProjectName]] setup"
  post-use:
    - composer install
    - npm install
    - |
      git init
      git add -A
      git commit -m "Initial commit: [[.ProjectName]]"
```

Each entry is a string passed to `bash -c`. Multiline strings (YAML `|`) are valid.

### Directory (`hooks/`)

```
<template-root>/
└── hooks/
    ├── pre-use.sh
    └── post-use.sh
```

Each file's content is executed via `bash <file>`. The context is injected as env vars the
same way as inline hooks. `[[ ]]` expressions in script files are **not** rendered — use env
vars for dynamic values (they are already injected).

---

## Files

### `pkg/hooks/hooks.go`

```go
package hooks

import (
    "bytes"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "text/template"

    "github.com/specsnl/specs-cli/pkg/specs"
    "github.com/specsnl/specs-cli/pkg/util/output"
    "github.com/specsnl/specs-cli/pkg/template" // for FuncMap
)

// Hooks holds the pre-use and post-use command lists.
type Hooks struct {
    PreUse  []string // each entry: a single command or multiline bash script
    PostUse []string
}

// Load reads hook definitions from templateRoot.
// Sources:
//   - Inline: the "hooks" key in projectConfig (parsed from project.yaml)
//   - Directory: hooks/pre-use.sh and hooks/post-use.sh under templateRoot
//
// Returns an error if both sources are present.
// Returns an empty *Hooks (not nil) if no hooks are defined at all.
func Load(templateRoot string, projectConfig map[string]any) (*Hooks, error) {
    hasInline := false
    var h Hooks

    if raw, ok := projectConfig["hooks"]; ok {
        if err := h.parseInline(raw); err != nil {
            return nil, fmt.Errorf("parsing inline hooks: %w", err)
        }
        hasInline = true
    }

    hooksDir := filepath.Join(templateRoot, "hooks")
    hasDir := dirExists(hooksDir)

    if hasInline && hasDir {
        return nil, specs.ErrBothHookSources
    }

    if hasDir {
        if err := h.loadFromDir(hooksDir); err != nil {
            return nil, err
        }
    }

    return &h, nil
}

// Run executes all commands for the given trigger ("pre-use" or "post-use").
// cwd is the working directory for the subprocess.
// ctx values are injected as UPPER_SNAKE_CASE env vars.
// [[ ]] template expressions in each command are rendered against ctx before execution.
// Returns immediately on the first non-zero exit.
func (h *Hooks) Run(trigger, cwd string, ctx map[string]any, funcMap template.FuncMap) error {
    var commands []string
    switch trigger {
    case "pre-use":
        commands = h.PreUse
    case "post-use":
        commands = h.PostUse
    default:
        return fmt.Errorf("unknown hook trigger: %q", trigger)
    }

    env := buildEnv(ctx)

    for _, cmdTpl := range commands {
        rendered, err := renderCommand(cmdTpl, ctx, funcMap)
        if err != nil {
            return fmt.Errorf("rendering hook command: %w", err)
        }

        output.Debug("running %s hook: %s", trigger, firstLine(rendered))

        cmd := exec.Command("bash", "-c", rendered)
        cmd.Dir = cwd
        cmd.Env = append(os.Environ(), env...)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr

        if err := cmd.Run(); err != nil {
            return fmt.Errorf("%s hook failed: %w", trigger, err)
        }
    }
    return nil
}

// HasPreUse reports whether any pre-use hooks are defined.
func (h *Hooks) HasPreUse() bool { return len(h.PreUse) > 0 }

// HasPostUse reports whether any post-use hooks are defined.
func (h *Hooks) HasPostUse() bool { return len(h.PostUse) > 0 }
```

#### Private helpers

```go
// parseInline decodes the raw "hooks" value from project.yaml into h.PreUse / h.PostUse.
// Expected shape:
//   hooks:
//     pre-use:  ["cmd1", "cmd2"]
//     post-use: ["cmd1"]
func (h *Hooks) parseInline(raw any) error {
    m, ok := raw.(map[string]any)
    if !ok {
        return fmt.Errorf("hooks must be a mapping, got %T", raw)
    }
    if pre, ok := m["pre-use"]; ok {
        cmds, err := toStringSlice(pre)
        if err != nil {
            return fmt.Errorf("pre-use: %w", err)
        }
        h.PreUse = cmds
    }
    if post, ok := m["post-use"]; ok {
        cmds, err := toStringSlice(post)
        if err != nil {
            return fmt.Errorf("post-use: %w", err)
        }
        h.PostUse = cmds
    }
    return nil
}

// loadFromDir reads hooks/pre-use.sh and hooks/post-use.sh if they exist.
// Each file's entire content becomes a single command entry.
func (h *Hooks) loadFromDir(hooksDir string) error {
    for _, name := range []struct {
        file    string
        target  *[]string
    }{
        {"pre-use.sh", &h.PreUse},
        {"post-use.sh", &h.PostUse},
    } {
        path := filepath.Join(hooksDir, name.file)
        data, err := os.ReadFile(path)
        if os.IsNotExist(err) {
            continue
        }
        if err != nil {
            return err
        }
        *name.target = []string{string(data)}
    }
    return nil
}

// buildEnv converts a context map to a slice of "KEY=value" strings.
// Keys are uppercased; non-string values are formatted with fmt.Sprintf.
func buildEnv(ctx map[string]any) []string {
    env := make([]string, 0, len(ctx))
    for k, v := range ctx {
        env = append(env, fmt.Sprintf("%s=%v", strings.ToUpper(k), v))
    }
    return env
}

// renderCommand renders [[ ]] template expressions in a hook command string.
func renderCommand(cmdTpl string, ctx map[string]any, funcMap template.FuncMap) (string, error) {
    if !strings.Contains(cmdTpl, "[[") {
        return cmdTpl, nil // fast path: no template expressions
    }
    tmpl, err := template.New("").Delims("[[", "]]").Funcs(funcMap).Parse(cmdTpl)
    if err != nil {
        return "", err
    }
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, ctx); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// toStringSlice coerces a YAML []any into []string.
func toStringSlice(v any) ([]string, error) {
    list, ok := v.([]any)
    if !ok {
        return nil, fmt.Errorf("expected a list, got %T", v)
    }
    result := make([]string, len(list))
    for i, item := range list {
        s, ok := item.(string)
        if !ok {
            return nil, fmt.Errorf("item %d is not a string: %T", i, item)
        }
        result[i] = s
    }
    return result, nil
}

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
    info, err := os.Stat(path)
    return err == nil && info.IsDir()
}

// firstLine returns the first line of a string, for log messages.
func firstLine(s string) string {
    if i := strings.IndexByte(s, '\n'); i >= 0 {
        return s[:i] + " …"
    }
    return s
}
```

---

## Tests

All tests use `t.TempDir()` and never require `bash` to run a real side effect. Subprocess
tests use simple, safe commands (`echo`, `exit`).

### `pkg/hooks/hooks_test.go`

#### Load — inline hooks

| Test | Setup | Expected |
|------|-------|----------|
| `TestLoad_InlinePreUse` | `hooks: {pre-use: ["echo hi"]}` | `h.PreUse == ["echo hi"]` |
| `TestLoad_InlinePostUse` | `hooks: {post-use: ["npm install"]}` | `h.PostUse == ["npm install"]` |
| `TestLoad_InlineBothTriggers` | pre and post both defined | both slices populated correctly |
| `TestLoad_NoHooks` | no hooks key, no hooks dir | returns empty `*Hooks`, no error |
| `TestLoad_EmptyHooks` | `hooks: {}` | returns empty `*Hooks`, no error |

#### Load — directory hooks

| Test | Setup | Expected |
|------|-------|----------|
| `TestLoad_DirPreUse` | `hooks/pre-use.sh` present | `h.PreUse` contains file content |
| `TestLoad_DirPostUse` | `hooks/post-use.sh` present | `h.PostUse` contains file content |
| `TestLoad_DirMissingFile` | `hooks/` exists, no `pre-use.sh` | `h.PreUse` is nil, no error |

#### Load — conflict

| Test | Setup | Expected |
|------|-------|----------|
| `TestLoad_BothSources` | inline hooks in yaml AND `hooks/` dir present | returns `ErrBothHookSources` |

#### Run

| Test | Setup | Expected |
|------|-------|----------|
| `TestRun_ExecutesCommand` | `PostUse: ["echo ok"]` | exits without error |
| `TestRun_NonZeroExitReturnsError` | `PostUse: ["exit 1"]` | returns non-nil error |
| `TestRun_StopsOnFirstFailure` | `PostUse: ["exit 1", "echo second"]` | error after first; second not run |
| `TestRun_InjectsEnvVars` | `PostUse: ["test \"$PROJECTNAME\" = acme"]`, `ctx["ProjectName"]="acme"` | exits 0 |
| `TestRun_RendersTemplateInCommand` | `PostUse: ["echo [[.Name]]"]`, `ctx["Name"]="world"` | exits 0 |
| `TestRun_UnknownTrigger` | trigger `"invalid"` | returns error immediately |
| `TestRun_EmptyHooks` | `Run("post-use", ...)` on empty `*Hooks` | returns nil |

---

## Key notes

- **`bash` is required.** Hook commands are always run via `bash -c`. If `bash` is not on
  `PATH`, `exec.Command` will return an error. This is acceptable — `specs` targets developer
  machines where bash is always present (Linux, macOS). A clear error message covers the edge case.
- **Script files are not template-rendered.** Only inline hook command strings support
  `[[ ]]` expressions. Script files in `hooks/` rely on the injected env vars instead. This
  avoids ambiguity and keeps `.sh` files valid shell regardless of sprout function names.
- **`cmd.Stdout = os.Stdout`** — hook output is streamed live to the terminal, not buffered.
  Template authors expect to see `composer install` output in real time.
- **Context env var format:** keys are uppercased with `strings.ToUpper`. This is simple and
  predictable: `ProjectName` → `PROJECTNAME`. Template authors use env vars in `.sh` files
  and inline commands accordingly.
- **`funcMap` parameter on `Run`:** hooks receive the same FuncMap as the template engine.
  This means sprout functions are available in inline hook command templates:
  `["cp -r dist [[toKebabCase .ProjectName]]"]`
- **`--no-hooks` flag** (Phase 6): the command layer checks this flag and simply skips calling
  `hooks.Run()`. No change needed in the hooks package itself.
- **Hook working directories:**
  - `pre-use`: the template source directory (where `project.yaml` lives)
  - `post-use`: the target directory (where the rendered files were written)
