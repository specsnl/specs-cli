# Boilr v2 — Library Decisions

## CLI Framework: Cobra (retained)

**Decision:** Keep `github.com/spf13/cobra`.

**Rationale:**
- Boilr's command tree (`boilr template download|save|use|list|…`) is already nested and will grow in v2.
- Cobra handles arbitrary subcommand depth cleanly via `AddCommand()`; flags can appear before or after arguments.
- Best-in-class shell completion for bash, zsh, fish, and PowerShell — important for a developer tool.
- Battle-tested in Kubernetes, Hugo, and GitHub CLI; no risk of the framework being a bottleneck.

**Alternative considered:** `github.com/urfave/cli`
Rejected because: its declarative struct style becomes awkward at depth, and its shell completion support has known gaps for nested command flags. Its main advantage (zero dependencies) is not a priority here.

---

## Interactive Prompts: huh

**Decision:** `charm.land/huh/v2` — full replacement for `pkg/prompt`.

**Rationale:**
- Huh is usable in **standalone mode** (`form.Run()` blocks like a normal function call), so it drops into any Cobra command with zero architectural change.
- Covers every prompt type the current `pkg/prompt` implements, plus more:

  | huh field | Replaces |
  |-----------|---------|
  | `Input` | `strPrompt` |
  | `Confirm` | `boolPrompt` (yes/no) |
  | `Select` | `multipleChoicePrompt` |
  | `MultiSelect` | *(not supported today)* |
  | `Text` | *(multi-line, not supported today)* |

- Built-in theming (Charm, Dracula, Catppuccin, Base16, Default) — no custom styling code needed.
- First-class accessibility mode for screen readers.
- `huh.Form` is also a `tea.Model`, so if boilr ever adopts a full Bubbletea TUI, the prompt code requires no rewrite.

**Migration path:**
Remove `pkg/prompt` entirely. Each command that currently calls `prompt.New(...)` will instead build a `huh.NewForm(...)` and call `.Run()`.

---

## Output Styling: lipgloss

**Decision:** `charm.land/lipgloss/v2` — replaces `fatih/color` and `tablewriter`.

**Rationale:**
- CSS-like chainable API for colour, bold/italic/underline, padding, margins, borders, and alignment.
- Handles colour downsampling automatically (24-bit → 8-bit → 4-bit based on terminal capability).
- Works standalone; no Bubbletea required.
- `pkg/util/tlog` and `pkg/util/tabular` can both be rewritten on top of lipgloss with less code than today.

**Libraries replaced:**
- `github.com/fatih/color` — lipgloss covers all colour output.
- `github.com/olekukonko/tablewriter` — lipgloss has a first-class table layout model.

---

## TUI Components: bubbles (selective)

**Decision:** `charm.land/bubbles/v2` — adopt individual components as needed.

**Initial candidates:**

| Component | Use case |
|-----------|---------|
| `spinner` | Show activity during `git clone` and template execution |
| `progress` | File copy progress for large templates |
| `table` | Replace `pkg/util/tabular` in the `list` command |

**Not adopting immediately:** `list`, `filepicker`, `viewport` — these require the full Bubbletea event loop and are deferred.

---

## TUI Framework: bubbletea (deferred)

**Decision:** `github.com/charmbracelet/bubbletea` — do not adopt in v2 initial scope.

**Rationale:**
- Bubbletea requires taking over the application's event loop; every interaction becomes a message-passing state machine. This is a significant architectural commitment.
- Huh already covers the interactive prompt UX. Bubbles spinner/progress work without the full event loop.
- Deferring keeps v2 tractable. Because huh forms are `tea.Model` implementations and bubbles components are designed for Bubbletea, adopting it later requires no rewrites — only a new top-level `Program`.

**Trigger for adoption:** If v2 introduces a browsable template registry UI, a live template preview, or any persistent screen-redraw requirement.

---

## Config / Context Parsing: go-yaml

**Decision:** `gopkg.in/yaml.v3` — replaces `encoding/json` for reading `project.yaml`.

**Rationale:**
- `project.json` is replaced by `project.yaml` in v2.
- YAML supports comments, making template config files self-documenting.
- `gopkg.in/yaml.v3` unmarshals into `map[string]interface{}` identically to `encoding/json`,
  so the internal context representation is unchanged.
- Standard, stable library — used across the Go ecosystem.

**Watch out for:** YAML's implicit type coercion. Unquoted values that look like numbers or
booleans are parsed as their native type. Template authors must quote strings that could be
misread (e.g. PHP versions: `"8.4"` not `8.4`, or YAML will parse it as `float64`).

**Backward compatibility:** v2 will also check for `project.json` if `project.yaml` is not
found, so existing templates continue to work.

---

## Libraries Removed from v1

| Library | Reason |
|---------|--------|
| `github.com/fatih/color` | Replaced by lipgloss |
| `github.com/olekukonko/tablewriter` | Replaced by lipgloss tables |

---

## Libraries Retained from v1

| Library | Reason |
|---------|--------|
| `github.com/spf13/cobra` | CLI framework — no change |
| `github.com/Masterminds/sprig` | Template functions — still needed |
| `github.com/go-git/go-git/v5` | Git clone — still needed |
| `github.com/sethvargo/go-password` | Password generation in FuncMap |
| `github.com/docker/go-units` | File size formatting in FuncMap |
| `github.com/ryanuber/go-glob` | Glob matching |

---

## Full Dependency Picture (v2 target)

```
cobra          CLI command tree
huh            interactive forms & prompts
lipgloss       output styling
bubbles        spinner, progress, table components
go-yaml v3     project.yaml parsing
sprig          extended template functions
go-git         git clone for template download
go-password    password() template function
go-units       formatFilesize() template function
go-glob        glob pattern matching
xdg            config/data directory resolution
```
