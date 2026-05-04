# Specs CLI — Library Decisions

## CLI Framework: Cobra (retained)

**Decision:** Keep `github.com/spf13/cobra`.

**Rationale:**
- The command tree (`specs template download|save|use|list|…`) is nested and will grow.
- Cobra handles arbitrary subcommand depth cleanly via `AddCommand()`; flags can appear before or after arguments.
- Best-in-class shell completion for bash, zsh, fish, and PowerShell.
- Battle-tested in Kubernetes, Hugo, and GitHub CLI.

---

## Interactive Prompts: huh

**Decision:** `charm.land/huh/v2` — full replacement for the old `pkg/prompt`.

**Rationale:**
- Usable in standalone mode (`form.Run()` blocks like a normal function call).
- Covers every original prompt type, plus more:

  | huh field | Replaces |
  |-----------|---------|
  | `Input` | `strPrompt` |
  | `Confirm` | `boolPrompt` (yes/no) |
  | `Select` | `multipleChoicePrompt` |
  | `MultiSelect` | *(not supported in v1)* |

- Built-in theming (Charm, Dracula, Catppuccin, Base16, Default).
- `huh.Form` is also a `tea.Model`, so adopting a full Bubbletea TUI later requires no prompt rewrites.

---

## Output Styling: lipgloss

**Decision:** `charm.land/lipgloss/v2` — replaces `fatih/color` and `tablewriter`.

**Rationale:**
- CSS-like chainable API for colour, bold/italic/underline, padding, margins, borders, and alignment.
- Handles colour downsampling automatically (24-bit → 8-bit → 4-bit based on terminal capability).
- `pkg/util/output` provides the logger and table renderer on top of lipgloss.

**Libraries replaced:**
- `github.com/fatih/color`
- `github.com/olekukonko/tablewriter`

---

## TUI Components: bubbles (indirect)

`charm.land/bubbles/v2` is pulled in transitively by huh but is not used directly.

The `specs template list` table renderer (`pkg/util/output/table.go`) is implemented
using lipgloss only — auto-sized columns, styled headers, and a border rendered with
lipgloss styles. No bubbles table component is used.

Future candidates if a full TUI is ever adopted:

| Component | Potential use case |
|-----------|---------|
| `spinner` | Activity indicator during git clone / template execution |
| `progress` | File copy progress for large templates |

Full Bubbletea event loop adoption is deferred — huh already covers the interactive prompt UX.

---

## Template Functions: sprout

**Decision:** `github.com/go-sprout/sprout` — replaces `github.com/Masterminds/sprig/v3`.
Backwards compatibility layer is **not** used.

**Rationale:**
- Sprig is effectively unmaintained; sprout is its active successor.
- Functions are grouped into opt-in registries — only pull in what is needed.
- `env` and `expandenv` are not included by default — templates cannot read host environment
  variables, reducing the attack surface for untrusted template downloads.
- Canonical function names follow Go conventions.

---

## Debug Logging: slog

**Decision:** `log/slog` (standard library, Go ≥ 1.21).

**Rationale:**
- Zero additional dependency.
- Structured key-value fields give context to debug messages.
- Silent by default; activated by `--debug` on the root command.

---

## Config / Context Parsing: go-yaml

**Decision:** `gopkg.in/yaml.v3` — replaces `encoding/json` for reading `project.yaml`.

**Rationale:**
- YAML supports comments, making template config files self-documenting.
- `gopkg.in/yaml.v3` unmarshals into `map[string]any` identically to `encoding/json`.
- Also used for `--values` files: `.yaml`/`.yml` extensions are parsed as YAML,
  all other extensions (e.g. `.json`) fall back to `encoding/json`.

**Watch out for:** YAML's implicit type coercion. Always quote strings that look like numbers:
`"8.4"` not `8.4` (YAML would parse the latter as `float64`).

**Backward compatibility:** `project.json` is still supported as a fallback.
`project.yaml` and `project.yml` are mutually exclusive — having both is an error.

---

## XDG Base Directories: adrg/xdg

**Decision:** `github.com/adrg/xdg`.

```go
// Respects $XDG_CONFIG_HOME, falls back to ~/.config/specs
configDir := filepath.Join(xdg.ConfigHome, "specs")
```

---

## Version Comparison: Masterminds/semver

**Decision:** `github.com/Masterminds/semver/v3` for semver-aware version comparison.

**Rationale:**
- `specs template upgrade` compares local and remote tag versions to find the highest
  available semver tag greater than the currently installed version.
- `semver.NewVersion()` + `GreaterThan()` replaces hand-rolled string comparison.

---

## SSH Authentication: golang.org/x/crypto

**Decision:** `golang.org/x/crypto` for SSH host key verification via `knownhosts.New`.

**Rationale:**
- go-git requires an explicit `HostKeyCallback` for SSH connections.
- `golang.org/x/crypto/ssh/knownhosts` reads `~/.ssh/known_hosts` and builds a callback
  that prevents MITM attacks.
- SSH agent and standard key files (`id_ed25519`, `id_rsa`, `id_ecdsa`) are tried in order.

---

## Full Dependency Picture

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI command tree |
| `charm.land/huh/v2` | Interactive forms & prompts |
| `charm.land/lipgloss/v2` | Output styling (logger + table renderer) |
| `gopkg.in/yaml.v3` | `project.yaml` parsing and `--values` YAML files |
| `github.com/go-sprout/sprout` | Extended template functions |
| `github.com/go-git/go-git/v5` | Git clone for template download/upgrade |
| `github.com/adrg/xdg` | Config/data directory resolution |
| `github.com/danwakefield/fnmatch` | Glob matching for `.specsverbatim` |
| `github.com/Masterminds/semver/v3` | Semver comparison for `template upgrade` |
| `github.com/sethvargo/go-password` | `password()` template function |
| `github.com/docker/go-units` | `formatFilesize()` template function |
| `golang.org/x/crypto` | SSH host key verification via `~/.ssh/known_hosts` |
| `log/slog` | Internal debug logging (stdlib) |
