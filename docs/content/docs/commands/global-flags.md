---
title: Global Flags
weight: 3
next: /docs/template-structure
---

| Flag                | Description                                                                       |
|---------------------|-----------------------------------------------------------------------------------|
| `--output` / `-o`   | Output format: `pretty` (default, styled) or `json` (NDJSON, one object per line) |
| `--debug`           | Enable debug-level logging                                                        |
| `--safe-mode`       | Disable env/filesystem functions and skip hooks                                   |
| `--no-env-prefix`   | Remove the `SPECS_` prefix from hook environment variables                        |
| `--non-interactive` | Never prompt; fail naming the missing values instead of asking for them           |

`--output` accepts `pretty` and `json` and nothing else. Any other value is an error, so a typo
fails at the flag rather than several lines later in the pipeline that consumes the output:

```console
$ specs --output josn version
error invalid --output "josn": want "pretty" or "json"
$ echo $?
1
```

`--non-interactive` is the explicit form of something specs infers anyway: a prompt is only drawn
when stdin is a terminal. The flag exists for the reverse case — checking at a terminal that a
command will not stall in CI. See [Running without a terminal](use#running-without-a-terminal).

## Which stream output lands on

stdout carries the **product** — the answer to the command, in whichever format `--output` selects.
stderr carries the **narration**: progress, confirmations, warnings and errors. So discarding stderr
leaves exactly the data behind, in both formats:

```console
$ specs template list --output json 2>/dev/null | jq -r .name
$ specs version --output json 2>/dev/null
{"version":"v0.0.13"}
```

Commands that only act on the filesystem (`template save`, `template download`, `use`, …) narrate
what they did and write nothing to stdout. `--debug` logging is separate again and always goes to
stderr.

`json` is NDJSON throughout — **one object per line**, a table row included, so a killed or failed
run still leaves every completed row readable. The keys are snake_case and independent of the
column headings the pretty table prints, and each value keeps its own type: a count is a number, a
timestamp is a timestamp, and a field with no value is absent rather than the `-` the table shows.

## Pretty tables

A pretty table is capped to the width of your terminal: when it does not fit, its widest columns
shrink and their cells wrap onto extra lines rather than the table breaking apart. Redirect stdout
to a file or a pipe and the full natural width is written instead — set `COLUMNS` to pin a width
there:

```sh
COLUMNS=100 specs template list | less -R
```

Cells that stand for a URL are printed as a **label** and hyperlinked to the full value, using
[OSC 8](https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda). Terminals that support
hyperlinks (iTerm2, WezTerm, kitty, Ghostty, GNOME Terminal, Windows Terminal, …) make the label
clickable, and it stays one link even when the column wraps it over several lines. Terminals
without support simply show the label, and a redirect to a file or a pipe writes plain text.
`--output json` always carries the value as stored, never the label.
