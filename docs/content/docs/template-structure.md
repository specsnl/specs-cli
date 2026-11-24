---
title: Template Structure
weight: 5
prev: /docs/commands
---

A template is a directory with this layout:

```text
my-template/
├── project.yml         # Variable schema, defaults, and hooks
└── template/           # Files and directories to render
    ├── {{ .projectName }}/
    │   └── main.go
    └── README.md
```

Both a project file (`project.yml`) and a `template/` directory are required.

## Template delimiters

Templates use `{{ }}` by default — standard Go `text/template` syntax:

```text
Hello, {{ .projectName }}!
```

All standard Go template syntax works inside `{{ }}`, including `if`, `range`, `with`, and pipes.

Directory and file names are also templated:

```text
{{ .projectName }}/
  {{ if .useDocker }}Dockerfile{{ end }}
  main.go
```

A rendered name must stay inside the generated project. A name that renders to an absolute path or
climbs out with `..` (`{{ .Dir }}/x.txt` with `Dir: ../../escaped`) is never written, whether the
value comes from a default, `--arg`, `--values` or a prompt. A `..` that stays inside is fine:
`a/../b/x.txt` is written as `b/x.txt`.

- For a local template, the file is skipped with a warning naming the source and rendered path. A
  directory is skipped with everything under it.
- A template cloned from a remote (`specs use <git-url>`, or one added with `specs template download`)
  fails instead, naming every such path, and nothing is written to the target.
- `specs template validate` reports these paths, rendered with the schema defaults, and exits with
  bit `16` set.

To avoid conflicts with tools that also use `{{ }}` (e.g. GitHub Actions, Helm), add `__delimiters` to `project.yml` to use a custom pair:

```yaml
__delimiters:
  left: "[["
  right: "]]"
```

With `[[ ]]` configured, `{{ }}` in your template files passes through unchanged.

## Reserved keys

Keys prefixed with `__` in the project file are reserved by `specs` and are never exposed as template variables:

| Key                | Purpose                                                                                                                                                                                                                                                                                                                                    |
|--------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `__delimiters`     | Override the default `{{ }}` delimiters (see above).                                                                                                                                                                                                                                                                                       |
| `__specs__version` | Declare a semver constraint on the `specs` CLI version required to use the template, e.g. `__specs__version: ^0.1.0`. `specs use` and `specs template use` refuse to run the template when the running binary does not satisfy the constraint (development builds are exempt). See _The project file_ page for accepted constraint shapes. |

The whole `__` namespace is reserved. You cannot define a variable or computed value whose name
starts with `__` unless it is one of the recognised configuration keys above. Doing so makes
`specs template download`, `specs template use`, and `specs template validate` fail with a
"reserved" error, keeping the namespace free for future configuration. The same rule applies to
values passed at run time via `--arg` or a `--values` file.

Even though they are reserved, the recognised keys' values are made available to your templates
under their original name, so you can reference them directly — for example
`{{ .__specs__version }}` or `{{ .__delimiters.left }}`.

## File permissions

File permission bits are always preserved from template source to output. A script
marked executable (`chmod +x`) in the template directory — or in the local registry —
will remain executable in every scaffolded project.

## Symlinks

A symlink in `template/` stays a symlink in the generated project, as long as it points at
something inside `template/`. This covers links to files and to directories, and chains of links:

```text
template/
├── .nvmrc
└── .node-version → .nvmrc        # generated as .node-version → .nvmrc
```

The link target is rendered like a file name, so `.node-version → {{ .Name }}.rc` points at the
rendered `acme.rc`.

specs never reads through a link. These links are skipped with a warning:

- links whose target is absolute, or climbs out of `template/` (`../`, `../../.ssh/id_rsa`), at any
  step of the chain
- links whose target does not exist
- links whose rendered target was not generated, for example because a conditional removed the file
- links that cannot be created, for example on Windows without the privilege to create symlinks

A template cloned from a remote (`specs use <git-url>`, `specs template download`,
`specs template upgrade`) with a link out of `template/` is rejected outright: the command fails,
naming the link and its target, and nothing is written or registered.

`specs template validate` reports skipped links and exits with bit `8` set.

## Skipping binary files

Create a `.specsverbatim` file in the template root to list glob patterns for files that should be copied as-is without template rendering:

```text
*.png
*.jpg
*.gif
*.woff2
dist/**
```

## Source formats

The `<source>` argument in `specs use` and `specs template download` accepts:

| Format           | Example                          |
|------------------|----------------------------------|
| GitHub shorthand | `user/repo`                      |
| GitHub + branch  | `user/repo:main`                 |
| HTTPS URL        | `https://github.com/user/repo`   |
| SSH (SCP-style)  | `git@github.com:user/repo`       |
| SSH URL          | `ssh://git@github.com/user/repo` |
| Local path       | `./path/to/template`             |
| Local (explicit) | `file:./path/to/template`        |

GitHub is the default host, so a bare `owner/repo` needs no prefix; the older `github:user/repo` form is still accepted as a deprecated alias. Because the shorthand is matched after every URL and path form, a relative path needs its leading `./` — a bare `templates/go` is read as a repository.

Local paths are only accepted by `specs use`. To register a local directory as a named template, use `specs template save` instead.

SSH clones authenticate automatically via SSH agent (if `SSH_AUTH_SOCK` is set) or standard key files (`~/.ssh/id_ed25519`, `id_rsa`, `id_ecdsa`). Host key verification uses `~/.ssh/known_hosts`.
