# Specs CLI

- **Binary:** `specs`
- **Module:** `github.com/specsnl/specs-cli`

## Conventions

- **Template delimiters:** always include a space after `[[` and before `]]`.
  Write `[[ .Variable ]]`, `[[ if .Flag ]]`, never `[[.Variable]]`.

- **Pipe operator:** always use the pipe operator to pass a value into a function.
  Write `[[ .Name | toKebabCase ]]`, never `[[ toKebabCase .Name ]]`.
  For chained transforms: `[[ .Name | toSnakeCase | toUpperCase ]]`.

- **Code changes require tests, docs, and README updates:** whenever code is added, removed,
  or updated, the following must happen in the same change:
  - **Tests** — add or update `*_test.go` files covering the changed behaviour.
  - **Docs** — update the relevant file(s) under `docs/` if the change affects
    package structure, data flows, CLI flags, configuration, or any documented design decision.
  - **README** — update `README.md` if the change affects anything user-facing: commands,
    flags, template syntax, source formats, functions, or storage layout.

## Architecture documentation

Architecture documentation lives in the `docs/` directory.

| File | Description |
|------|-------------|
| [docs/architecture/overview.md](./docs/architecture/overview.md) | Package structure, CLI tree, data flows |
| [docs/architecture/template-engine.md](./docs/architecture/template-engine.md) | Template engine: delimiters, verbatim copy, conditional files, hooks |
| [docs/architecture/computed-values.md](./docs/architecture/computed-values.md) | Computed values: post-prompt derived context keys |
| [docs/architecture/library-decisions.md](./docs/architecture/library-decisions.md) | Library choices and rationale |
| [docs/operations/release.md](./docs/operations/release.md) | Release pipeline: GoReleaser, GitHub Releases, Homebrew, CI/CD |
