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

## Development

**Requirements:** [Task](https://taskfile.dev) and Docker — build and test commands run inside a Docker container, so no local Go installation is needed. If you prefer to run Go commands directly on your host instead, Go 1.26+ is required.

Build the Docker images first (one-time setup):

```sh
task dc:build
```

Then:

```sh
task build    # Build the binary for the current platform
task test     # Run unit tests
```

List all available tasks:

```sh
task --list
```

---

## License

MIT — see [LICENSE](./LICENSE).
