---
title: Global Flags
weight: 3
next: /docs/template-structure
---

| Flag              | Description                                                                 |
|-------------------|-----------------------------------------------------------------------------|
| `--output` / `-o` | Output format: `pretty` (default, styled) or `json` (NDJSON, for scripting) |
| `--debug`         | Enable debug-level logging                                                  |
| `--safe-mode`     | Disable env/filesystem functions and skip hooks                             |
| `--no-env-prefix` | Remove the `SPECS_` prefix from hook environment variables                  |

## Which stream output lands on

stdout carries the **product** — the answer to the command, in whichever format `--output` selects.
stderr carries the **narration**: progress, confirmations, warnings and errors. So discarding stderr
leaves exactly the data behind, in both formats:

```console
$ specs template list --output json 2>/dev/null | jq '.[].Name'
$ specs version --output json 2>/dev/null
{"version":"v0.0.13"}
```

Commands that only act on the filesystem (`template save`, `template download`, `use`, …) narrate
what they did and write nothing to stdout. `--debug` logging is separate again and always goes to
stderr.
