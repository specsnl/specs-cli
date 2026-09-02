---
title: Output
weight: 5
---

## The contract

**stdout carries the product, stderr carries the narration.**

stdout is what a caller redirects to a file or pipes into another command — the answer to the
question the command was asked. Everything else — progress, confirmations, warnings, errors — is
narration and belongs on stderr. That split is what keeps `2>/dev/null` meaningful and what makes a
pipeline composable:

```console
$ specs template list --output json 2>/dev/null | jq '.[].Name'
$ specs version --output json 2>/dev/null
{"version":"v0.0.13"}
```

Because the narration never reaches stdout, `specs version` stays usable as `$(specs version)` and
`specs template use … > out.txt` captures nothing it should not.

---

## The `Writer` interface

All user-facing output goes through `output.Writer` (`internal/util/output/writer.go`):

```go
type Writer interface {
    Info(format string, args ...any)
    Warn(format string, args ...any)
    Error(format string, args ...any)
    // WriteErr renders err as an error message; JSON output includes error_kind when known.
    WriteErr(err error)
    // Table renders rows under headers. A Cell's Value is what JSON emits and
    // its Text what the pretty writer displays. Wrap plain rows with Rows.
    Table(headers []string, rows [][]Cell)
    // WriteResult renders a single-line result on stdout: the product of a command
    // whose answer is not a table.
    WriteResult(record any, format string, args ...any)
}
```

| Method        | Stream | Role      | Pretty                                      | JSON                                         |
|---------------|--------|-----------|---------------------------------------------|----------------------------------------------|
| `Table`       | stdout | Product   | Styled table of each cell's `Text`          | One array of objects, of each cell's `Value` |
| `WriteResult` | stdout | Product   | The formatted text, unstyled and unprefixed | The marshalled record; the text is dropped   |
| `Info`        | stderr | Narration | `info …`                                    | `{"level":"info","message":…}`               |
| `Warn`        | stderr | Narration | `warn …`                                    | `{"level":"warn","message":…}`               |
| `Error`       | stderr | Narration | `error …`                                   | `{"level":"error","message":…}`              |
| `WriteErr`    | stderr | Narration | `error …`                                   | Adds `"error_kind"` for known sentinels      |

`WriteResult` exists so a result that is not a table still leaves a *typed* line on stdout: a
consumer reads `.version` instead of regexing a version out of an English sentence. The pretty
writer keeps the sentence; the JSON writer keeps the record and drops the text. A record JSON
cannot represent writes nothing rather than a broken line.

`JSONWriter.WriteErr` for a known sentinel:

```json
{"level":"error","message":"template not found: mytemplate","error_kind":"template_not_found"}
```

Two implementations are selected at startup via `--output`:

| Flag value         | Writer         | Behaviour                                                    |
|--------------------|----------------|--------------------------------------------------------------|
| `pretty` (default) | `PrettyWriter` | Lipgloss-styled text, one line (or table) per call           |
| `json`             | `JSONWriter`   | NDJSON: one bare JSON object per line, one array per `Table` |

### Deciding which method a command needs

- Does the caller want the value in a file or a pipe? → `Table` (many rows) or `WriteResult` (one
  answer).
- Is it about *what the command is doing to the system* — cloning, running a hook, saving,
  deleting? → `Info`.
- Is it an empty answer? Write the empty **product** anyway — `template list` writes an empty table
  when nothing is registered, so a consumer parses one document either way — and narrate the
  explanation with `Info`.

`slog` is a third channel and not part of this contract: a debug-only diagnostic stream on stderr.
See [Logging](overview#logging).

---

## Where the colour decision is made

`NewPrettyWriter(stdout, stderr, environ)` wraps **each** stream in a
`colorprofile.Writer`, so how much colour is emitted is decided per stream from the streams
themselves plus `environ` — not once per process. A terminal on stderr and a redirected stdout get
different answers, which is the ordinary case for `specs template list > out.txt`.

Pass `nil` for `environ` to use the process environment; `NewDefaultPrettyWriter()` does exactly
that. Tests pass an explicit slice so the rendered bytes do not depend on who runs the suite.

`CLICOLOR_FORCE` and `CLICOLOR` are honoured by `colorprofile`. `NO_COLOR` is applied by
`profileWriter` instead, because `colorprofile` honours it only for a stream that is itself a
terminal — `NO_COLOR=1 CLICOLOR_FORCE=1` into a pipe would otherwise still be coloured. It clamps to
the ASCII profile, which drops colour and keeps bold, as [no-color.org](https://no-color.org/) asks.

`JSONWriter` writes to the raw streams: JSON output is never styled.

---

## Where the width decision is made

`RenderTable(headers, rows, maxWidth)` renders with
[`charm.land/lipgloss/v2/table`](library-decisions#output-styling-lipgloss) and takes the width as a
**parameter** rather than detecting it — the same reasoning as `environ` above: the function stays
environment-free and its goldens stay deterministic.

`maxWidth` is a **cap, never a stretch**. It is applied only when the natural table — every column
sized to its widest cell — is wider than the space available, so a short table keeps its compact
look and the output does not change with every terminal size. When it does apply, lipgloss shrinks
the widest columns first (by median non-whitespace length) and wraps **data** cells onto extra
lines. Headers are never wrapped, so a very narrow window truncates header text; that is preferable
to the previous behaviour, where the *border* wrapped and the table fell apart into fragments.

`PrettyWriter.Table` resolves the width **per call**, so a terminal resized between two commands is
honoured:

| Order | Source                                           | Applies to                            |
|-------|--------------------------------------------------|---------------------------------------|
| 1     | `term.GetSize` on stdout, when stdout is a tty   | An interactive terminal               |
| 2     | A positive `COLUMNS` from the captured `environ` | The escape hatch for pipes and tests  |
| 3     | `0` — unconstrained                              | A file or a pipe, which wants it full |

`PrettyWriter` therefore keeps stdout *before* the `colorprofile` wrapper, so a file descriptor is
still reachable for step 1, alongside the `environ` slice step 2 reads.

Cells are measured by `ansi.StringWidth`, so display width and embedded ANSI escapes are both
handled: `café`, `日本語` and `🚀` occupy the cells they actually occupy on screen, and a pre-styled
cell is not counted as wide as its escape bytes.

`JSONWriter` is unaffected: JSON output is never width-dependent.

---

## Hyperlinked cells

A cell with a `Link`, or a **data** cell whose `Value` is entirely an `http(s)` URL, is rendered as
an [OSC 8 hyperlink](https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda), so a
`Repository` opens on a click instead of leaving the reader to select and copy it. Headers are never
linked; nor is anything a terminal could not open — an SSH remote (`git@host:path`), a filesystem
path, the `-` placeholder, or prose that merely contains a URL.

The interesting part is a URL the width cap has wrapped. Every segment carries the **same `id=`**
parameter, which is what the OSC 8 spec asks for: segments sharing an id are one logical link, so
hovering highlights all of them and a click anywhere opens the whole URL. lipgloss closes the
sequence before the padding on each line, so the border is never swallowed into the link. The id is
`<row>-<col>`, unique per cell, so two rows pointing at the same repository still highlight
separately.

Linking is **not** gated on detecting terminal support — there is no reliable query for it, and none
is needed:

| Destination                                  | Result                                          |
|----------------------------------------------|-------------------------------------------------|
| A terminal that supports OSC 8               | A clickable link                                |
| A terminal that does not (e.g. Terminal.app) | The sequence is ignored; the URL text is shown  |
| A file or a pipe                             | `colorprofile.Writer` strips it; plain URL text |
| `CLICOLOR_FORCE=1` into a file or a pipe     | Kept, exactly as forced colour is               |

That last row is the same trade colour already makes, and the third is why `RenderTable` can emit
the sequences unconditionally: the per-stream `colorprofile.Writer` that strips colour for a
non-terminal stream strips OSC 8 with it, so `specs template list > out.txt` writes a clean file.

Linking lives in `RenderTable`, **not** at the call site that builds the rows. `JSONWriter` copies
a cell's `Value` straight into a JSON value, so a cell linkified upstream would put escape bytes
inside `"Repository"` and corrupt `specs template list -o json`.

---

## The `Cell` seam: a label is not the value

`Cell` carries the two forms of a table cell apart:

| Field   | Read by        | Purpose                                                      |
|---------|----------------|--------------------------------------------------------------|
| `Value` | `JSONWriter`   | What a consumer parses — never shortened, never decorated    |
| `Text`  | `PrettyWriter` | What a reader sees; falls back to `Value` via `Cell.Label()` |
| `Link`  | `PrettyWriter` | The URL `Text` is hyperlinked to; overrides auto-detection   |

`Cell{Value: x}` is the ordinary cell where the two coincide, and `output.Rows` wraps plain
`[][]string` for a table that needs nothing more — `template update` uses it.

This is the same split `WriteResult(record, format, args…)` already makes: one machine form, one
human form, each writer taking what it needs.

### Why the shortening is not in this package

`specs template list` renders `https://github.com/specsnl/specs-cli` as `specsnl/specs-cli` and a
saved path as `~/code/proto-template`. Those rules live in `internal/cmd/repo_display.go`, not here,
because this package encodes only what is true of *terminals* — rune width, whether a stream renders
colour, that `http(s)://` is openable — while every shortening rule is knowledge about what a
`Repository` value *means*:

- `local:` is a marker this CLI invented for `__metadata.json`; nothing else would recognise it.
- Dropping `github.com` is a product decision — GitHub is this CLI's default host.
- The `~` collapse reads `$HOME`, and a renderer that reads the environment could not have a
  deterministic golden. That is the same reason `maxWidth` is a parameter.

So `output` renders and `cmd` decides what a value means. `Cell` is the seam, and it is what keeps
the shortened label out of `--output json`.

---

## Testing the rendering

Rendering is long, mostly whitespace, and tedious to assert field by field — so it is asserted
against golden files in `internal/util/output/testdata/*.golden` rather than by hand. Each case also
names the stream it expects, and the test fails if anything reaches the other one — that assertion
is what pins the contract above.

| Command            | Effect                                                                 |
|--------------------|------------------------------------------------------------------------|
| `task test`        | Compares the current output against the checked-in `.golden` files     |
| `task test:update` | Rewrites the `.golden` files from the current output — review the diff |

`golden_test.go` pins two environments so the files are reproducible wherever the suite runs:

| Environment    | Value                                                   | Renders                 |
|----------------|---------------------------------------------------------|-------------------------|
| `goldenPlain`  | `[]string{}`                                            | Every sequence stripped |
| `goldenColour` | `CLICOLOR_FORCE=1`, `TERM=xterm`, `COLORTERM=truecolor` | Full colour             |

`COLORTERM=truecolor` is deliberate: it short-circuits `colorprofile.Detect` before the terminfo
lookup, which reads the system terminfo database and would answer differently on a developer's
machine than in the `go-builder` container.

Table cases also name a `maxWidth`, including one capped well below the natural width and one above
it, so the wrapping and the "cap only, never stretch" rule each have a golden of their own.
`table_hyperlink` and `table_hyperlink_wrapped` pin the OSC 8 sequences, the latter across a
wrapped URL where the shared `id=` is the whole point. `table_shortened_labels` pins the shape
`template list` produces: a short label carrying the full URL as its target, beside an unlinked
path.

One assertion freezes behaviour that is known to be wrong, so the issue that fixes it produces a
reviewable diff: `Table` emitting one JSON array rather than NDJSON (#109). It is marked as such in
the tests.
