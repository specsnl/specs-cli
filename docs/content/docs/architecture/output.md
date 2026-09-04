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
$ specs template list --output json 2>/dev/null | jq -r .name
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
    // WriteTable renders a table: the pretty writer draws its cells, the JSON
    // writer marshals its records one per line. Build the TableData with the
    // generic Table function rather than calling this directly.
    WriteTable(data TableData)
    // WriteResult renders a single-line result on stdout: the product of a command
    // whose answer is not a table.
    WriteResult(record any, format string, args ...any)
}
```

| Method        | Stream | Role      | Pretty                                      | JSON                                           |
|---------------|--------|-----------|---------------------------------------------|------------------------------------------------|
| `WriteTable`  | stdout | Product   | Styled table of each cell's `Text`          | One JSON object per line, of each row's record |
| `WriteResult` | stdout | Product   | The formatted text, unstyled and unprefixed | The marshalled record; the text is dropped     |
| `Info`        | stderr | Narration | `info …`                                    | `{"level":"info","message":…}`                 |
| `Warn`        | stderr | Narration | `warn …`                                    | `{"level":"warn","message":…}`                 |
| `Error`       | stderr | Narration | `error …`                                   | `{"level":"error","message":…}`                |
| `WriteErr`    | stderr | Narration | `error …`                                   | Adds `"error_kind"` for known sentinels        |

`WriteResult` exists so a result that is not a table still leaves a *typed* line on stdout: a
consumer reads `.version` instead of regexing a version out of an English sentence. The pretty
writer keeps the sentence; the JSON writer keeps the record and drops the text. A record JSON
cannot represent writes nothing rather than a broken line.

`JSONWriter.WriteErr` for a known sentinel:

```json
{"level":"error","message":"template not found: mytemplate","error_kind":"template_not_found"}
```

Two implementations are selected at startup via `--output`, whose accepted values are the
`output.Format` constants rather than bare string literals:

| `Format`       | Flag value         | Writer         | Behaviour                                               |
|----------------|--------------------|----------------|---------------------------------------------------------|
| `FormatPretty` | `pretty` (default) | `PrettyWriter` | Lipgloss-styled text, one line (or table) per call      |
| `FormatJSON`   | `json`             | `JSONWriter`   | NDJSON: one bare JSON object per line, for every method |

`Format.Valid()` is what `PersistentPreRunE` rejects an unknown value with; there is no fallback
arm that quietly renders a typo as pretty.

The writer is wired **before** the format is validated. That ordering reads backwards, and is
deliberate: `main` reports the returned error through `app.Output`, so a rejection raised while
`app.Output` is still nil would have nowhere to go. Wiring an invalid format as pretty costs
nothing, because the next line rejects it.

### Deciding which method a command needs

- Does the caller want the value in a file or a pipe? → `Table` (many rows) or `WriteResult` (one
  answer).
- Is it about *what the command is doing to the system* — cloning, running a hook, saving,
  deleting? → `Info`.
- Is it an empty answer? Write the **product** anyway and narrate the explanation with `Info`.
  `template list` calls `Table` with no rows when nothing is registered: the pretty writer draws
  the headers, so a reader sees the shape of the answer, and the JSON writer writes nothing, which
  is what an empty NDJSON document is.

`slog` is a third channel and not part of this contract: a debug-only diagnostic stream on stderr.
`SetupLogger` in `log.go` is the only place its handler is built, and its default level is
`LevelSilent`, so nothing reaches a user who did not ask for it. See [Logging](overview#logging).

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

`PrettyWriter.WriteTable` resolves the width **per call**, so a terminal resized between two commands is
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

Linking lives in `RenderTable`, **not** at the call site that builds the rows. A cell linkified
upstream would carry escape bytes into whatever else reads it, and it would be doing terminal work
in a place that knows nothing about terminals. `JSONWriter` never sees a cell at all — it reads the
records — so `specs template list -o json` is safe either way.

---

## The `Cell` seam: a label is not the value

`Cell` carries the two forms of a table cell apart:

| Field   | Read by        | Purpose                                                      |
|---------|----------------|--------------------------------------------------------------|
| `Value` | `JSONWriter`   | What a consumer parses — never shortened, never decorated    |
| `Text`  | `PrettyWriter` | What a reader sees; falls back to `Value` via `Cell.Label()` |
| `Link`  | `PrettyWriter` | The URL `Text` is hyperlinked to; overrides auto-detection   |

`Cell{Value: x}` is the ordinary cell where the two coincide, and `output.Col` builds one from a
row without the caller naming the type. `output.Rows` wraps plain `[][]string` for a direct
`RenderTable` call.

This is the same split `WriteResult(record, format, args…)` already makes: one machine form, one
human form, each writer taking what it needs.

### Why the shortening is not in this package

`specs template list` renders `https://github.com/specsnl/specs-cli` as `specsnl/specs-cli` and
normalises a saved path to `~/code/proto-template`, whichever form the metadata carries. Those rules live in `internal/cmd/repo_display.go`, not here,
because this package encodes only what is true of *terminals* — rune width, whether a stream renders
colour, that `http(s)://` is openable — while every shortening rule is knowledge about what a
`Repository` value *means*:

- Telling a stored path from a git URL is a fact about `__metadata.json`, not about terminals.
- Dropping `github.com` is a product decision — GitHub is this CLI's default host.
- The `~` collapse reads `$HOME`, and a renderer that reads the environment could not have a
  deterministic golden. That is the same reason `maxWidth` is a parameter.

So `output` renders and `cmd` decides what a value means. `Cell` is the seam, and it is what keeps
the shortened label out of `--output json`.

---

## Building a table: two audiences, two inputs

A column heading and a JSON key look like the same string and are not. A heading is prose for a
reader and may be reworded freely; a key is what a consumer's `jq` filter matches on. While one
string did both jobs, `Table` could only take `[][]string` — so every value reached JSON as a
display string, `12` arrived as `"12"`, and rewording `Name` would have silently started returning
`null` downstream.

Giving the two audiences separate inputs answers the key-naming question by itself:

```go
type templateRow struct {
    Name    string `json:"name"`
    Version string `json:"version"`
    Updates int    `json:"updates_available"`
}

output.Table(w, rows,
    output.Col("Name", func(r templateRow) string { return r.Name }),
    output.Col("Version", func(r templateRow) string { return r.Version }),
    output.Col("Updates available", func(r templateRow) string { return strconv.Itoa(r.Updates) }),
)
```

| Type              | Role                                                                        |
|-------------------|-----------------------------------------------------------------------------|
| `Column[T]`       | One column: the `Header` a reader sees, and a `Cell` function for a row     |
| `Col` / `ColCell` | Build a column from a plain value, or from a `Cell` with its own label/link |
| `TableData`       | `Headers` + `Cells` for the reader, `Records` for the consumer              |
| `Table[T]`        | The entry point — derives both forms from the same rows                     |

`Table` is a free function rather than a `Writer` method because Go has no generic methods; the
interface carries `WriteTable(TableData)` underneath. Going through it is also what guarantees
every row has exactly one cell per header — alignment a raw `[][]Cell` can silently get wrong.

The keys are the row type's `json` tags, so `Updates available` can be reworded without touching
`updates_available`, and a number stays a number:

```console
specs template list -o json 2>/dev/null | jq 'select(.updates_available > 0)'
```

### The breaking part

`-o json` table output changed shape in three ways at once, because none of them could be fixed
alone:

| Before                             | After                                      |
|------------------------------------|--------------------------------------------|
| One JSON array for the whole table | One object per line — genuinely NDJSON     |
| Keys from headings (`Name`)        | Keys from `json` tags (`name`)             |
| Every value a string (`"12"`)      | The row's own types (`12`, `true`, a time) |

A pipeline reading `jq '.[].Name'` becomes `jq -r .name`. `template list` also now emits the
timestamps themselves rather than the `3 days ago` the table shows, and omits a field it has no
value for instead of writing the `-` placeholder, which is a display convention.

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

`json_table` pins the shape a row type produces: snake_case keys from its `json` tags and a count
that is a number, not `"3"`. `json_table_empty` pins the empty NDJSON document — no bytes at all.
