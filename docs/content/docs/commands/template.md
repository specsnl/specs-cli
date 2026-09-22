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
| `update [name]`                              | Force-refresh the cached update status and report a `Name`/`Status`/`Latest` table; updates all template statuses if no name is given                                                                               |
| `upgrade [name]`                             | Apply available updates; upgrades all templates if no name is given (remote templates re-clone, local templates re-copy from their source path)                                                                     |

`template use` accepts the same flags as `specs use` (`--values`, `--arg`, `--use-defaults`, `--no-hooks`).

`template download` accepts the same source formats as `specs use` — see [Source formats](/docs/commands/use/#source-formats).

`template download` and `template save` accept `-f` / `--force` to overwrite an existing template with the same name.

For machine-readable `list` and `update` output, use the global `--output json` flag (see
[Global Flags](/docs/commands/global-flags/)). Their tables are the product and go to stdout — an empty registry
still yields `[]`, with the explanation narrated on stderr — while `validate` answers
`{"valid": true|false}` and `version` answers `{"version": "…"}`.

## The Repository column

`template list` prints a **label** in its `Repository` column, not the raw stored value. A GitHub
URL reads as `specsnl/specs-cli`, since GitHub is the default host; any other host keeps its name
(`gitlab.com/acme/tpl`); and a saved path collapses `$HOME` to `~`. The label is clickable in
terminals that support hyperlinks — see [Pretty tables](/docs/commands/global-flags/#pretty-tables).

`--output json` carries the value as stored, so scripts read the full URL:

```sh
specs template list -o json 2>/dev/null | jq -r .repository
```

For a template registered with `template save`, that value is the source path with your home
directory written as `~` (e.g. `~/code/my-template`). Templates saved by older versions carry a
`local:` prefix instead; that form is still read, and migrates on the next `template upgrade`.

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

`template update` reports the same labels, except that it carries the newly found version in its own
`Latest` column rather than inside the status string, and lists only the templates it actually
checked — untracked ones are left out instead of shown as `-`.

Cached statuses are stored per template and re-checked when older than 24 hours **or** when they
were written by a different `specs` version — so an update to the status-check logic takes effect
on the next `list` after upgrading the CLI, rather than after the cache expires.
