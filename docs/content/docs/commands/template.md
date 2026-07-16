---
title: specs template
weight: 2
---

Manage a local registry of named templates. Unlike `specs use`, downloaded templates are stored persistently and can be reused.

## Subcommands

| Subcommand                                   | Description                                                                                                                                                                                                         |
|----------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `list` / `ls`                                | List registered templates with update status. Cached statuses are refreshed automatically when older than 24 hours or written by a different specs version. Use `--output json` for machine-readable NDJSON output. |
| `save <path> <name>`                         | Register a local directory as a template                                                                                                                                                                            |
| `download` / `add <source> <name>`           | Download a remote template and save it to the local registry                                                                                                                                                        |
| `use <name> <target-dir>`                    | Execute a registered template                                                                                                                                                                                       |
| `validate <path>`                            | Check if a template directory is valid                                                                                                                                                                              |
| `rename` / `mv <old> <new>`                  | Rename a registered template                                                                                                                                                                                        |
| `delete` / `rm` / `remove` / `del <name>...` | Remove one or more templates from the registry                                                                                                                                                                      |
| `update [name]`                              | Force-refresh the cached update status; updates all template statuses if no name is given                                                                                                                           |
| `upgrade [name]`                             | Apply available updates; upgrades all templates if no name is given (remote templates re-clone, local templates re-copy from their source path)                                                                     |

`template use` accepts the same flags as `specs use` (`--values`, `--arg`, `--use-defaults`, `--no-hooks`).

`template download` accepts the same source formats as `specs use` — see [Source formats](use#source-formats).

`template download` and `template save` accept `-f` / `--force` to overwrite an existing template with the same name.

For machine-readable `list` output, use the global `--output json` flag (see [Global Flags](global-flags)).

## Update status

The `Status` column reflects where each template's "source of truth" lives:

- **Remote templates** (downloaded from a git URL) are compared against the remote. For tagged
  templates a newer version means a strictly-greater semver tag; for branch-tracked templates
  sitting on a released tag the same semver rule applies, otherwise the branch tip is compared.
- **Local templates** (registered with `save`) are compared against the **source directory on
  disk**, not a git remote. `update available` means the source path has moved ahead of the
  commit/version recorded when the template was saved. Uncommitted changes in the source (a
  "dirty" working tree) do **not** count as an update on their own: while the source stays on the
  saved commit, the transient `-dirty` marker on its git-describe version is ignored so the
  template is not perpetually reported as out of date. If the source path no longer exists the
  status is `source missing`.

Possible `Status` values: `up-to-date`, `update: <version>`, `update available`,
`unknown (offline?)`, `auth error`, `not found`, `source missing`, `unknown`, `-` (untracked).

Cached statuses are stored per template and re-checked when older than 24 hours **or** when they
were written by a different `specs` version — so an update to the status-check logic takes effect
on the next `list` after upgrading the CLI, rather than after the cache expires.
