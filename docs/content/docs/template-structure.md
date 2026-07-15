---
title: Template Structure
weight: 4
prev: /docs/commands
---

A template is a directory with this layout:

```
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

```
Hello, {{ .projectName }}!
```

All standard Go template syntax works inside `{{ }}`, including `if`, `range`, `with`, and pipes.

Directory and file names are also templated:

```
{{ .projectName }}/
  {{ if .useDocker }}Dockerfile{{ end }}
  main.go
```

To avoid conflicts with tools that also use `{{ }}` (e.g. GitHub Actions, Helm), add `__delimiters` to `project.yml` to use a custom pair:

```yaml
__delimiters:
  left: "[["
  right: "]]"
```

With `[[ ]]` configured, `{{ }}` in your template files passes through unchanged.

## Reserved keys

Keys prefixed with `__` in the project file are reserved by `specs` and are never exposed as template variables:

| Key | Purpose |
|-----|---------|
| `__delimiters` | Override the default `{{ }}` delimiters (see above). |
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

## Skipping binary files

Create a `.specsverbatim` file in the template root to list glob patterns for files that should be copied as-is without template rendering:

```
*.png
*.jpg
*.gif
*.woff2
dist/**
```

## Source formats

The `<source>` argument in `specs use` and `specs template download` accepts:

| Format | Example |
|--------|---------|
| GitHub shorthand | `github:user/repo` |
| GitHub + branch | `github:user/repo:main` |
| HTTPS URL | `https://github.com/user/repo` |
| SSH (SCP-style) | `git@github.com:user/repo` |
| SSH URL | `ssh://git@github.com/user/repo` |
| Local path | `./path/to/template` |
| Local (explicit) | `file:./path/to/template` |

Local paths are only accepted by `specs use`. To register a local directory as a named template, use `specs template save` instead.

SSH clones authenticate automatically via SSH agent (if `SSH_AUTH_SOCK` is set) or standard key files (`~/.ssh/id_ed25519`, `id_rsa`, `id_ecdsa`). Host key verification uses `~/.ssh/known_hosts`.
