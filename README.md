<p align="center">
  <img src="docs/static/logo.svg" width="200" alt="Specs CLI">
  <h1 align="center">Specs CLI</h1>
  <p align="center"><strong>Documentation:</strong> <a href="https://cli.specs.dev">cli.specs.dev</a></p>
</p>

A general-purpose developer CLI for scaffolding projects from templates. Define variables, write template files, run hooks — `specs` handles the rest.

---

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

## The project file

A template's `project.yml` declares its variables, defaults, computed values, and hooks. A few `__`-prefixed keys are reserved by `specs` and never exposed as template variables:

| Key | Purpose |
|-----|---------|
| `__delimiters` | Override the default `{{ }}` template delimiters with a custom pair (e.g. `[[ ]]`). |
| `__specs__version` | Declare a [semver](https://github.com/Masterminds/semver) constraint on the `specs` CLI version required to use the template, e.g. `__specs__version: ^0.1.0`. `specs use` and `specs template use` refuse to run the template unless the running binary satisfies the constraint (development builds are exempt; `specs template save` skips the check). |

See the [documentation](https://cli.specs.dev) for the full project-file reference.

---

## Development environment

### Overview

The repo ships a `Dockerfile` and a `compose.yml` that together define a self-contained build and test environment. Contributors don't need a local Go installation — all builds and tests run inside a Docker container that pins the exact Go version and tooling.

| File | Role |
|------|------|
| `Dockerfile` | Defines the build image — Go 1.26 + tooling, used by `task build` and `task test` |
| `compose.yml` | Wires the Dockerfile stages into named services consumed by the Taskfile |
| `Taskfile.dist.yml` | Orchestrates all developer workflows; wraps Docker Compose so you never call it directly |

**Requirements:** [Task](https://taskfile.dev) and Docker.

### Getting started

Build the images once before running any task:

```sh
task dc:build
```

Then use the standard tasks:

```sh
task build    # Build the binary for the current platform
task test     # Run unit tests
```

List all available tasks:

```sh
task --list
```

### How it works

`task dc:build` builds all Docker Compose services in the `build` profile. The key service is `go-builder`, built from the `builder-download` stage of the `Dockerfile`. It mounts the repository root and two Docker volumes — one for the Go module cache and one for the build cache — so subsequent runs are fast.

`task test` and `task build` spin up a one-off `go-builder` container (`docker compose run --rm`), run the Go command inside it, then discard the container. The service doesn't need to be started in advance — it is ephemeral by design.

`task build` also invokes `docker buildx bake` using the `go-binary` service to produce a statically linked binary and copy it out of the image into the project root.

### Without Docker (escape hatch)

If you already have Go 1.26+ installed locally, you can bypass the container entirely:

```sh
go build ./...
go test ./...
```

CI always runs through Docker and the Taskfile. The container is the source of truth for reproducible builds.

### CI and agent execution

All CI and agent workflows follow the same rule: use `task` commands, never call `docker compose` directly. See [`.github/instructions/executing-commands.md`](.github/instructions/executing-commands.md) for the authoritative execution rules.

---

## License

MIT — see [LICENSE](./LICENSE).
