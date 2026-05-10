---
title: specs template
weight: 2
---

Manage a local registry of named templates. Unlike `specs use`, downloaded templates are stored persistently and can be reused.

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` / `ls` | List registered templates with update status. Stale statuses (older than 24 hours) are refreshed automatically. Use `--output json` for machine-readable NDJSON output. |
| `save <path> <name>` | Register a local directory as a template |
| `download <source> <name>` | Download a remote template and save it to the local registry |
| `use <name> <target-dir>` | Execute a registered template |
| `validate <path>` | Check if a template directory is valid |
| `rename` / `mv <old> <new>` | Rename a registered template |
| `delete` / `rm` / `remove` / `del <name>...` | Remove one or more templates from the registry |
| `update [name]` | Force-refresh the cached update status; updates all template statuses if no name is given |
| `upgrade [name]` | Apply available updates; upgrades all remote templates if no name is given |

`template use` accepts the same flags as `specs use` (`--values`, `--arg`, `--use-defaults`, `--no-hooks`).

`template download` accepts the same source formats as `specs use` — see [Source formats](use#source-formats).

`template download` and `template save` accept `-f` / `--force` to overwrite an existing template with the same name.

For machine-readable `list` output, use the global `--output json` flag (see [Global Flags](global-flags)).
