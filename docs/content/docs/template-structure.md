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
