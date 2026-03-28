# Phase 1 — Project Skeleton

## Goal

A compilable binary that responds to `specs --help` and `specs version`. No business logic yet
— just the Cobra scaffolding and the `template` subcommand group as an empty parent.

## Done criteria

- `go build ./...` succeeds with no errors.
- `specs --help` exits 0 and lists the available commands.
- `specs version` prints a version string.
- All tests pass.

---

## Dependencies

```
go mod init github.com/specsnl/specs-cli
go get github.com/spf13/cobra
```

---

## File overview

```
specs-cli/
├── main.go
└── pkg/
    └── cmd/
        ├── root.go
        ├── template.go
        └── version.go
```

---

## Files

### `main.go`

Single responsibility: call `cmd.Execute()` and exit.

```go
package main

import (
    "os"

    "github.com/specsnl/specs-cli/pkg/cmd"
)

func main() {
    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

No logic lives here. `Execute()` returns any unhandled error; the caller decides the exit code.

---

### `pkg/cmd/root.go`

Defines the root Cobra command and the public `Execute()` function.

```go
package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
    Use:   "specs",
    Short: "General-purpose developer CLI",
    Long: `specs is a multi-purpose developer CLI.

Use "specs <command> --help" for more information about a command.`,
    SilenceUsage:  true,
    SilenceErrors: true,
}

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    rootCmd.AddCommand(templateCmd)
    rootCmd.AddCommand(versionCmd)
}
```

**`SilenceUsage: true`** — Cobra's default prints usage on every error, which is noisy. We
only want usage shown when an unknown command or missing argument is detected, not on runtime
errors inside a command.

**`SilenceErrors: true`** — We handle error printing ourselves (via `output.Error`), so Cobra
should not print the error a second time.

---

### `pkg/cmd/template.go`

The `template` subcommand group. It has no `Run` of its own — it is a pure parent command
that groups all `template *` subcommands.

```go
package cmd

import "github.com/spf13/cobra"

var templateCmd = &cobra.Command{
    Use:   "template",
    Short: "Manage project templates",
    Long:  "Download, save, list, use, and manage project templates.",
}
```

Subcommands (`template list`, `template use`, etc.) are registered here in later phases via
`templateCmd.AddCommand(...)` calls inside their own `init()` functions.

---

### `pkg/cmd/version.go`

Prints the version string. `--dont-prettify` is reserved for Phase 2 when lipgloss is
available; for now it's accepted but has no visual effect.

```go
package cmd

import (
    "fmt"

    "github.com/spf13/cobra"
)

// Version is the current binary version.
// Set at build time via: -ldflags "-X github.com/specsnl/specs-cli/pkg/cmd.Version=1.0.0"
var Version = "dev"

var dontPrettifyVersion bool

var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print the specs version",
    Args:  cobra.NoArgs,
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Fprintf(cmd.OutOrStdout(), "specs version %s\n", Version)
        return nil
    },
}

func init() {
    versionCmd.Flags().BoolVar(&dontPrettifyVersion, "dont-prettify", false,
        "Print plain text without styling")
}
```

`cmd.OutOrStdout()` is used instead of `fmt.Println` so that tests can inject a buffer and
capture output without redirecting `os.Stdout`.

---

## Tests

File: `pkg/cmd/root_test.go`

Use a helper to execute commands and capture output:

```go
package cmd_test

import (
    "bytes"
    "testing"

    "github.com/specsnl/specs-cli/pkg/cmd"
    "github.com/spf13/cobra"
)

// executeRoot resets rootCmd state and executes it with the given args.
// Returns captured stdout and the execution error.
func executeRoot(args ...string) (string, error) {
    buf := new(bytes.Buffer)
    rootCmd.SetOut(buf)
    rootCmd.SetErr(buf)
    rootCmd.SetArgs(args)
    err := cmd.Execute()
    return buf.String(), err
}
```

> **Note:** `rootCmd` must be exported or the test helper must live in `package cmd`
> (not `cmd_test`) to access it. Keep tests in `package cmd` for this phase so they can
> reach unexported internals.

### Test cases

| Test | Args | Expected |
|------|------|----------|
| `TestHelpExitsZero` | `--help` | exits 0, output contains `"specs"` |
| `TestVersionCommand` | `version` | exits 0, output contains `"specs version"` |
| `TestVersionDontPrettify` | `version --dont-prettify` | exits 0, output contains `"specs version"` |
| `TestTemplateGroupHelp` | `template --help` | exits 0, output contains `"template"` |
| `TestUnknownCommandError` | `nonexistent` | exits non-zero |

### Example test

```go
func TestVersionCommand(t *testing.T) {
    out, err := executeRoot("version")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !strings.Contains(out, "specs version") {
        t.Errorf("expected output to contain 'specs version', got: %q", out)
    }
}
```

---

## Key notes

- **No `init` side effects in `main.go`.** All command wiring happens in `pkg/cmd` `init()`
  functions. `main.go` stays dumb.
- **`--dont-prettify` is command-local**, not a persistent root flag. Only commands that
  produce styled output expose it (`version`, `template list`).
- **Build version injection:** during development `Version = "dev"`. CI injects the real
  version at link time:
  ```
  go build -ldflags "-X github.com/specsnl/specs-cli/pkg/cmd.Version=$(git describe --tags)"
  ```
