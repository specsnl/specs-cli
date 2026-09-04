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
