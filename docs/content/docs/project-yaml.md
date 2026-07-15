---
title: The project file
weight: 5
---

The project file is named `project.yml` — this is the preferred name. `project.yaml` and `project.json` are also accepted as fallbacks, but `project.yml` takes precedence.

It defines the variables your template accepts and their defaults, plus optional computed values and hooks.

Each variable is a top-level key whose value is the default. The prompt type is inferred from the YAML value type:

| YAML value | Prompt |
|------------|--------|
| `"my-app"` (string) | Text input |
| `false` / `true` (bool) | Yes/No confirm |
| `["MIT", "Apache-2.0"]` (array) | Select list |

For select lists, the first option is the default — it is pre-selected when prompting interactively and chosen automatically when using `--use-defaults`.

```yaml
# Variables — value is the default; type is inferred from the YAML value
projectName: "my-app"
useDocker: false
license:
  - MIT
  - Apache-2.0
  - GPL-2.0

# A string default can reference other variables using {{ }} expressions
dockerImage: "{{ hostname }}.azurecr.io/{{ .projectName }}"

# Computed values — derived after prompting, not shown to the user
computed:
  packagePath: "github.com/{{ username }}/{{ .projectName }}"
  year: "{{ now | date \"2006\" }}"

# Hooks — run before and after rendering
hooks:
  pre-use:
    - echo "Creating {{ .projectName }}..."
  post-use:
    - git init
    - go mod tidy
```

## Required specs version

Set the reserved `__specs__version` key to declare which `specs` CLI versions a template supports. When present, `specs use` and `specs template use` refuse to run the template unless the running binary satisfies the constraint:

```yaml
__specs__version: ^0.1.0
```

The value is any valid [Masterminds/semver](https://github.com/Masterminds/semver) constraint string — both lower and upper bounds are supported:

| Value | Meaning |
|-------|---------|
| `0.1.0` | exactly `0.1.0` |
| `^0.1.0` | `>= 0.1.0, < 0.2.0` |
| `^1.1.1` | `>= 1.1.1, < 2.0.0` |
| `~0.1.0` | `>= 0.1.0, < 0.2.0` |
| `>= 0.1.0` | `>= 0.1.0` |
| `>= 0.1.0, < 2.0` | explicit range |

A development build (version `dev`, the default when building from source) skips the check so contributors are never locked out. The key is reserved and never exposed as a template variable. `specs template save` intentionally does **not** run the check — you can save a newer template on an older CLI for later use.

## Computed values

Entries under `computed:` are evaluated after all prompting is complete. They add new keys to the template context and are never shown as prompts. Values may reference user-provided variables.

## Conditional prompting

`specs` analyzes your template files at runtime. Variables that only appear inside conditional blocks (`{{ if .someFlag }}`) are only prompted when their condition is actually satisfied — keeping the interactive flow focused.

## Hooks

Hooks run shell commands before (`pre-use`) or after (`post-use`) rendering. They have access to all template variables as environment variables (prefixed with `SPECS_` by default):

```sh
# Available in hooks:
# SPECS_PROJECTNAME=my-app
# SPECS_PACKAGEPATH=github.com/user/my-app
echo "Initializing $SPECS_PROJECTNAME"
git init
```

Hook commands may use `{{ }}` template expressions. To skip hooks for a single run, pass `--no-hooks`.

You can also define hooks as scripts in a `hooks/` directory at the template root:

```
my-template/
├── project.yml
├── template/
└── hooks/
    ├── pre-use.sh
    └── post-use.sh
```

Either inline YAML hooks or a `hooks/` directory may be used — not both.
