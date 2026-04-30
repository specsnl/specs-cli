# Specs CLI

A general-purpose developer CLI for scaffolding projects from templates. Define variables, write template files, run hooks — `specs` handles the rest.

- [Installation](#installation)
- [Quick start](#quick-start)
- [Commands](#commands)
  - [`specs use`](#specs-use-source-target-dir)
  - [`specs template`](#specs-template-subcommand)
  - [Global flags](#global-flags)
- [Template structure](#template-structure)
  - [Template delimiters](#template-delimiters)
- [project.yaml](#projectyaml)
  - [Computed values](#computed-values)
  - [Conditional prompting](#conditional-prompting)
  - [Hooks](#hooks)
- [Skipping binary files](#skipping-binary-files)
- [Source formats](#source-formats)
- [Template functions](#template-functions)
  - [Specs functions](#specs-functions)
  - [Sprout function categories](#sprout-function-categories)
- [Storage](#storage)
- [Development](#development)
- [License](#license)

## Installation

**Homebrew (macOS):**

```sh
brew install specsnl/tap/specs
```

**From source:**

```sh
go install github.com/specsnl/specs-cli@latest
```

**Download a binary** from the [releases page](https://github.com/specsnl/specs-cli/releases).

---

## Quick start

Use a template directly without registering it first:

```sh
specs use github:specsnl/my-template ./my-project
```

Or register a template and reuse it later:

```sh
specs template download github:specsnl/my-template my-template
specs template use my-template ./my-project
```

---

## Commands

### `specs use <source> <target-dir>`

Fetch a template from any source, execute it into `<target-dir>`, and discard the download. Nothing is added to the registry.

```sh
specs use github:specsnl/go-service ./new-service
specs use ./local-template ./output --use-defaults
specs use github:specsnl/go-service ./new-service --arg projectName=my-service
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--values <file>` | Load variable values from a JSON file |
| `--arg <key=value>` | Set a single variable (repeatable) |
| `--use-defaults` | Accept all defaults without prompting |
| `--no-hooks` | Skip pre/post-use hooks |

### `specs template <subcommand>`

Manage a local registry of named templates.

| Subcommand | Description |
|------------|-------------|
| `list` / `ls` | Show all registered templates with update status |
| `save <path> <name>` | Register a local directory as a template |
| `download <source> <name>` | Clone a remote repo and register it |
| `use <name> <target-dir>` | Execute a registered template |
| `validate <path>` | Check if a template directory is valid |
| `rename` / `mv <old> <new>` | Rename a registered template |
| `delete` / `rm <name>...` | Remove one or more templates from the registry |
| `update [name]` | Check for upstream updates |
| `upgrade <name>` / `--all` | Apply available updates |

### Global flags

| Flag | Description |
|------|-------------|
| `--debug` | Enable debug-level logging |
| `--safe-mode` | Disable env/filesystem functions and skip hooks |
| `--no-env-prefix` | Remove the `SPECS_` prefix from hook environment variables |

---

## Template structure

A template is a directory with this layout:

```
my-template/
├── project.yaml        # Variable schema, defaults, and hooks
└── template/           # Files and directories to render
    ├── [[ projectName ]]/
    │   └── main.go
    └── README.md
```

Both `project.yaml` (or `project.json`) and a `template/` directory are required.

### Template delimiters

Templates use `[[ ]]` instead of `{{ }}` to avoid conflicts with many common file formats:

```
Hello, [[ .projectName ]]!
```

All standard Go template syntax works inside `[[ ]]`, including `if`, `range`, `with`, and pipes.

Directory and file names are also templated:

```
[[ .projectName ]]/
  [[ if .useDocker ]]Dockerfile[[ end ]]
  main.go
```

---

## project.yaml

Defines the variables your template accepts and their defaults, plus optional computed values and hooks.

Each variable is a top-level key whose value is the default. The prompt type is inferred from the YAML value type:

| YAML value | Prompt |
|------------|--------|
| `"my-app"` (string) | Text input |
| `false` / `true` (bool) | Yes/No confirm |
| `["MIT", "Apache-2.0"]` (array) | Select list |

```yaml
# Variables — value is the default; type is inferred from the YAML value
projectName: "my-app"
useDocker: false
license:
  - MIT
  - Apache-2.0
  - GPL-2.0

# A string default can reference other variables using [[ ]] expressions
dockerImage: "[[ hostname ]].azurecr.io/[[ .projectName ]]"

# Computed values — derived after prompting, not shown to the user
computed:
  packagePath: "github.com/[[ username ]]/[[ .projectName ]]"
  year: "[[ now | date \"2006\" ]]"

# Hooks — run before and after rendering
hooks:
  pre-use:
    - echo "Creating [[ .projectName ]]..."
  post-use:
    - git init
    - go mod tidy
```

### Computed values

Entries under `computed:` are evaluated after all prompting is complete. They add new keys to the template context and are never shown as prompts. Values may reference user-provided variables.

### Conditional prompting

`specs` analyzes your template files at runtime. Variables that only appear inside conditional blocks (`[[ if .someFlag ]]`) are only prompted when their condition is actually satisfied — keeping the interactive flow focused and minimal.

### Hooks

Hooks run shell commands before (`pre-use`) or after (`post-use`) rendering. They have access to all template variables as environment variables (prefixed with `SPECS_` by default):

```sh
# Available in hooks:
# SPECS_PROJECTNAME=my-app
# SPECS_PACKAGEPATH=github.com/user/my-app
echo "Initializing $SPECS_PROJECTNAME"
git init
```

Hook commands may use `[[ ]]` template expressions, which are rendered before execution. To skip hooks for a single run, pass `--no-hooks`.

Alternatively, hooks can be defined as scripts in a `hooks/` directory at the template root (next to `project.yaml`):

```
my-template/
├── project.yaml
├── template/
└── hooks/
    ├── pre-use.sh
    └── post-use.sh
```

Either inline YAML hooks or a `hooks/` directory may be used — not both.

---

## Skipping binary files

Create a `.specsverbatim` file in the template root to list glob patterns for files that should be copied as-is without template rendering:

```
*.png
*.jpg
*.gif
*.woff2
dist/**
```

---

## Source formats

The `<source>` argument in `specs use` and `specs template download` accepts:

| Format | Example |
|--------|---------|
| GitHub shorthand | `github:user/repo` |
| GitHub + branch | `github:user/repo:main` |
| HTTPS URL | `https://github.com/user/repo` |
| SSH | `git@github.com:user/repo` |
| Local path | `./path/to/template` |
| Local (explicit) | `file:./path/to/template` |

---

## Template functions

Templates have access to 200+ functions provided by [Sprout](https://github.com/go-sprout/sprout), plus a set of specs-specific functions.

### Specs functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `hostname` | `hostname` → `string` | System hostname |
| `username` | `username` → `string` | Current OS username |
| `toBinary` | `toBinary <int>` → `string` | Integer to binary string |
| `formatFilesize` | `formatFilesize <bytes>` → `string` | Human-readable file size (e.g. `"1.0 MB"`) |
| `password` | `password <length> <digits> <symbols> <noUpper> <allowRepeat>` → `string` | Generate a secure random password |

**Examples:**

```
Default registry: [[ hostname ]].azurecr.io
Author: [[ username ]]
Secret key: [[ password 32 4 4 false false ]]
```

### Sprout function categories

Sprout organizes its functions into registries. All of the following are available in templates:

| Category | Example functions |
|----------|-------------------|
| **Strings** | `upper`, `lower`, `camelcase`, `snakecase`, `trim`, `replace`, `contains`, `repeat` |
| **Encoding** | `b64enc`, `b64dec`, `toJson`, `fromJson`, `toYaml`, `fromYaml` |
| **Regex** | `regexMatch`, `regexFind`, `regexReplaceAll` |
| **Collections** | `list`, `dict`, `append`, `prepend`, `uniq`, `keys`, `values`, `merge` |
| **Date & time** | `now`, `date`, `dateModify`, `dateAgo`, `duration` |
| **Identity** | `uuidv4`, `uuidv5` |
| **Crypto** | `sha256sum`, `sha1sum`, `md5sum`, `bcrypt` |
| **Numeric** | `add`, `sub`, `mul`, `div`, `mod`, `floor`, `ceil`, `round` |
| **Semver** | `semver`, `semverCompare` |
| **Network** | `getHostByName` |
| **Random** | `randInt`, `randAlpha`, `randAlphaNum`, `randAscii` |
| **Reflection** | `typeOf`, `kindOf`, `kindIs` |
| **Environment** | `env`, `expandenv` *(disabled in `--safe-mode`)* |
| **Filesystem** | `osBase`, `osDir`, `osExt` *(disabled in `--safe-mode`)* |

Full documentation for Sprout functions is available at [docs.gomsprout.dev](https://docs.gosprout.dev).

---

## Storage

Templates are stored under the XDG config directory (respects `$XDG_CONFIG_HOME`):

```
~/.config/specs/
└── templates/
    ├── my-template/
    └── another-template/
```

---

## Development

**Requirements:** Go 1.26+, Docker, [Task](https://taskfile.dev)

```sh
task build    # Build the binary
task test     # Run all tests
```

List all available tasks:

```sh
task --list
```

---

## License

MIT — see [LICENSE](./LICENSE).
