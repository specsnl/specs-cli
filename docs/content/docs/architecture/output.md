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
    Table(headers []string, rows [][]string)
    // WriteResult renders a single-line result on stdout: the product of a command
    // whose answer is not a table.
    WriteResult(record any, format string, args ...any)
}
```

| Method        | Stream | Role      | Pretty                                      | JSON                                       |
|---------------|--------|-----------|---------------------------------------------|--------------------------------------------|
| `Table`       | stdout | Product   | Styled table                                | One array of objects, keyed by header      |
| `WriteResult` | stdout | Product   | The formatted text, unstyled and unprefixed | The marshalled record; the text is dropped |
| `Info`        | stderr | Narration | `info …`                                    | `{"level":"info","message":…}`             |
| `Warn`        | stderr | Narration | `warn …`                                    | `{"level":"warn","message":…}`             |
| `Error`       | stderr | Narration | `error …`                                   | `{"level":"error","message":…}`            |
| `WriteErr`    | stderr | Narration | `error …`                                   | Adds `"error_kind"` for known sentinels    |

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

Two assertions freeze behaviour that is known to be wrong, so the issue that fixes it produces a
reviewable diff: column widths measured in bytes (#117) and `Table` emitting one JSON array rather
than NDJSON (#109). Both are marked as such in the tests.
