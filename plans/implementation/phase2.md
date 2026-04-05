# Phase 2 — Config & Output Infrastructure

## Goal

XDG paths resolved and typed as constants; sentinel errors defined; styled terminal output
(logger + table renderer) working. Nothing is printed to the user in this phase directly —
the infrastructure is built so later phases can use it.

## Done criteria

- `pkg/specs.ConfigDir()` returns the correct path when `XDG_CONFIG_HOME` is overridden.
- `pkg/specs.TemplateDir()` returns `<configDir>/templates`.
- `output.Info/Warn/Error/Debug` each produce non-empty, styled output.
- `output.RenderTable` produces a string containing header and row data.
- All tests pass.

---

## Dependencies

```
go get github.com/adrg/xdg
go get charm.land/lipgloss/v2
go get charm.land/bubbles/v2
```

---

## File overview

```
pkg/
├── specs/
│   ├── configuration.go
│   └── errors.go
└── util/
    ├── exit/
    │   └── exit.go
    └── output/
        ├── log.go
        └── table.go
```

---

## Files

### `pkg/specs/configuration.go`

Central home for all path logic and file-name constants. Every other package imports these
constants rather than hard-coding strings.

```go
package specs

import (
    "path/filepath"

    "github.com/adrg/xdg"
)

const (
    AppName         = "specs"
    TemplateDirName = "templates"

    // File names inside a template root
    ProjectYAMLFile = "project.yaml"
    ProjectJSONFile = "project.json" // backward-compat fallback
    MetadataFile    = "__metadata.json"
    IgnoreFile      = ".specsignore"
    TemplateDirFile = "template"     // the subdirectory that gets rendered
)

// ConfigDir returns the specs configuration directory.
// Defaults to $XDG_CONFIG_HOME/specs (~/.config/specs).
func ConfigDir() string {
    return filepath.Join(xdg.ConfigHome, AppName)
}

// TemplateDir returns the directory where registered templates are stored.
func TemplateDir() string {
    return filepath.Join(ConfigDir(), TemplateDirName)
}

// TemplatePath returns the full path to a specific registered template by tag.
func TemplatePath(tag string) string {
    return filepath.Join(TemplateDir(), tag)
}

// IsRegistryInitialised reports whether the template directory exists on disk.
func IsRegistryInitialised() bool {
    info, err := os.Stat(TemplateDir())
    return err == nil && info.IsDir()
}
```

**How `adrg/xdg` works:** `xdg.ConfigHome` is a package-level string variable set from the
`XDG_CONFIG_HOME` environment variable at startup, falling back to `~/.config`. It is not
a function call — overriding `XDG_CONFIG_HOME` in a test via `t.Setenv` is enough to change
what `ConfigDir()` returns (no need to stub anything).

---

### `pkg/specs/errors.go`

Sentinel errors for conditions that Cobra commands need to detect and report specifically.

```go
package specs

import "errors"

var (
    // ErrRegistryNotInitialised is returned when the template directory does not exist.
    // Fix: run `specs init`.
    ErrRegistryNotInitialised = errors.New("template registry is not initialised — run 'specs init'")

    // ErrTemplateNotFound is returned when a tag is given that has no matching directory.
    ErrTemplateNotFound = errors.New("template not found")

    // ErrTemplateAlreadyExists is returned on save/download when the tag is already in use
    // and --force was not passed.
    ErrTemplateAlreadyExists = errors.New("template already exists — use --force to overwrite")

    // ErrTemplateDirMissing is returned when the template root exists but has no template/ subdir.
    ErrTemplateDirMissing = errors.New("template directory is missing a 'template/' subdirectory")

    // ErrBothHookSources is returned when project.yaml contains inline hooks AND a hooks/
    // directory also exists. Only one source is allowed.
    ErrBothHookSources = errors.New("conflicting hook sources: found both inline hooks in project.yaml and a hooks/ directory")
)
```

---

### `pkg/util/exit/exit.go`

Exit code constants. Using named constants avoids magic numbers scattered across command files.

```go
package exit

const (
    OK    = 0
    Error = 1
)
```

---

### `pkg/util/output/log.go`

Lipgloss-based levelled logger. Each level has a styled prefix label. Info and debug go to
stdout; warn and error go to stderr.

```go
package output

import (
    "fmt"
    "io"
    "os"

    "charm.land/lipgloss/v2"
)

var (
    styleInfo  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(12)) // bright blue
    styleWarn  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(11)) // bright yellow
    styleError = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(9))  // bright red
    styleDebug = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8))             // dark grey
)

// debugEnabled can be toggled by a --debug persistent flag in Phase 6.
var debugEnabled bool

func SetDebug(enabled bool) { debugEnabled = enabled }

func Info(format string, a ...any) {
    logTo(os.Stdout, styleInfo.Render("info")+" ", format, a...)
}

func Warn(format string, a ...any) {
    logTo(os.Stderr, styleWarn.Render("warn")+" ", format, a...)
}

func Error(format string, a ...any) {
    logTo(os.Stderr, styleError.Render("error")+" ", format, a...)
}

func Debug(format string, a ...any) {
    if !debugEnabled {
        return
    }
    logTo(os.Stdout, styleDebug.Render("debug")+" ", format, a...)
}

func logTo(w io.Writer, prefix, format string, a ...any) {
    lipgloss.Fprintln(w, fmt.Sprintf(prefix+format, a...))
}
```

**lipgloss v2 — key differences from v1:**
- `lipgloss.Color("12")` (string type) is gone. Use `lipgloss.ANSIColor(n)` for indexed
  256-colour values, hex strings as `color.RGBA` for truecolour, or the 16 named constants
  (`lipgloss.BrightBlue`, `lipgloss.Red`, etc.).
- Colour downsampling now happens at the **output layer**, not the rendering layer. Use
  `lipgloss.Fprintln(w, text)` / `lipgloss.Println(text)` / `lipgloss.Sprint(text)` instead of
  `fmt.Fprintf` so that downsampling to 8-bit or 4-bit occurs automatically based on the
  terminal's capability. The `logTo` helper above uses `lipgloss.Fprintln` for this reason.
- The `Renderer` type and `DefaultRenderer()` are removed. `Style` is now a plain value type.

**`--dont-prettify` integration (Phase 6):** When this flag is set, commands that call
`output.Info/Warn/Error` should call the plain variants instead. A simple approach is to
add `PlainInfo`, `PlainWarn`, `PlainError` variants, or check a package-level `prettify bool`
flag. Defer this decision to Phase 6 when actual commands are written.

---

### `pkg/util/output/table.go`

Table renderer used by `specs template list`. Returns a fully formatted string — the caller
prints it.

```go
package output

import (
    "charm.land/bubbles/v2/table"
    "charm.land/lipgloss/v2"
)

var tableStyle = lipgloss.NewStyle().
    BorderStyle(lipgloss.NormalBorder()).
    BorderForeground(lipgloss.ANSIColor(240))

// RenderTable renders headers and rows as a styled table string.
// Column widths are computed automatically from content.
func RenderTable(headers []string, rows [][]string) string {
    cols := makeColumns(headers, rows)

    t := table.New(
        table.WithColumns(cols),
        table.WithRows(toTableRows(rows)),
        table.WithFocused(false),
        table.WithHeight(len(rows)),
    )

    s := table.DefaultStyles()
    s.Header = s.Header.
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.ANSIColor(240)).
        BorderBottom(true).
        Bold(true)
    s.Cell = s.Cell.Padding(0, 1)
    t.SetStyles(s)

    return tableStyle.Render(t.View())
}

func makeColumns(headers []string, rows [][]string) []table.Column {
    widths := make([]int, len(headers))
    for i, h := range headers {
        widths[i] = len(h)
    }
    for _, row := range rows {
        for i, cell := range row {
            if i < len(widths) && len(cell) > widths[i] {
                widths[i] = len(cell)
            }
        }
    }
    cols := make([]table.Column, len(headers))
    for i, h := range headers {
        cols[i] = table.Column{Title: h, Width: widths[i] + 2} // +2 for padding
    }
    return cols
}

func toTableRows(rows [][]string) []table.Row {
    result := make([]table.Row, len(rows))
    for i, r := range rows {
        result[i] = table.Row(r)
    }
    return result
}
```

---

## Tests

### `pkg/specs/configuration_test.go`

```go
func TestConfigDir_XDGOverride(t *testing.T) {
    tmp := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tmp)
    xdg.Reload()
    t.Cleanup(func() { xdg.Reload() })
    got := specs.ConfigDir()
    want := filepath.Join(tmp, "specs")
    if got != want {
        t.Errorf("ConfigDir() = %q, want %q", got, want)
    }
}

func TestTemplateDir_XDGOverride(t *testing.T) {
    tmp := t.TempDir()
    t.Setenv("XDG_CONFIG_HOME", tmp)
    xdg.Reload()
    t.Cleanup(func() { xdg.Reload() })
    got := specs.TemplateDir()
    want := filepath.Join(tmp, "specs", "templates")
    if got != want {
        t.Errorf("TemplateDir() = %q, want %q", got, want)
    }
}
```

> **`adrg/xdg` and env vars:** `xdg.ConfigHome` is evaluated once when the package is
> initialised. In tests, `t.Setenv` changes the env var but the xdg variable is already set.
> Call `xdg.Reload()` after `t.Setenv` and register `t.Cleanup(func() { xdg.Reload() })` to
> restore it. The library exposes `Reload()` for exactly this purpose.

### `pkg/util/output/log_test.go`

```go
func TestInfoOutputNonEmpty(t *testing.T) {
    // Redirect stdout temporarily
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    output.Info("hello %s", "world")

    w.Close()
    os.Stdout = old
    var buf bytes.Buffer
    buf.ReadFrom(r)

    if buf.Len() == 0 {
        t.Error("Info() produced no output")
    }
}
```

Test each level (Info, Warn, Error, Debug) similarly. For Debug, set `output.SetDebug(true)`
before calling.

### `pkg/util/output/table_test.go`

```go
func TestRenderTable_ContainsHeaders(t *testing.T) {
    out := output.RenderTable(
        []string{"Tag", "Repository", "Created"},
        [][]string{{"my-tag", "user/repo", "2 days ago"}},
    )
    if !strings.Contains(out, "Tag") {
        t.Error("table output does not contain header 'Tag'")
    }
    if !strings.Contains(out, "my-tag") {
        t.Error("table output does not contain row value 'my-tag'")
    }
}
```

---

## Key notes

- **`pkg/specs` is not `pkg/cmd`.** Keep configuration and error types in `pkg/specs` so the
  template engine and hook packages can import them without importing Cobra.
- **No `init()` in cmd packages yet.** Phase 2 only adds utility packages; no new Cobra
  commands are wired in this phase.
- **`IsRegistryInitialised()` needs `os` import** — add it to `configuration.go`.
- **Table column widths:** the `makeColumns` helper above auto-sizes to the widest content.
  This is acceptable for Phase 2; a more sophisticated adaptive layout can be added in Phase 6
  if needed.
