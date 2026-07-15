# Specs CLI

- **Binary:** `specs`
- **Module:** `github.com/specsnl/specs-cli`

## Conventions

- **Template delimiters:** the default delimiters are `{{` and `}}`. Always include a space
  after the opening delimiter and before the closing delimiter.
  Write `{{ .Variable }}`, `{{ if .Flag }}`, never `{{.Variable}}`.
  When a template uses custom delimiters (via `__delimiters` in `project.yaml`), follow the
  same spacing convention for whatever pair is configured.

- **Pipe operator:** always use the pipe operator to pass a value into a function.
  Write `{{ .Name | toKebabCase }}`, never `{{ toKebabCase .Name }}`.
  For chained transforms: `{{ .Name | toSnakeCase | toUpperCase }}`.

- **Code changes require tests, docs, and README updates:** whenever code is added, removed,
  or updated, the following must happen in the same change:
  - **Tests** — add or update `*_test.go` files covering the changed behaviour.
  - **Docs** — update the relevant file(s) under `docs/content/` if the change affects
    package structure, data flows, CLI flags, configuration, file-handling behaviour, or
    any documented design decision. The actual docs live at
    `docs/content/docs/architecture/` (internal) and `docs/content/docs/` (user-facing);
    the paths in the table below are shortcuts — always resolve them under `docs/content/`.
    _This step is mandatory and must not be skipped, even for "internal" fixes._
  - **README** — update `README.md` if the change affects anything user-facing: commands,
    flags, template syntax, source formats, functions, or storage layout.

## Architecture documentation

Architecture documentation lives in the `docs/` directory.

| File                                                                                                         | Description                                                                            |
|--------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------|
| [docs/content/docs/architecture/overview.md](./docs/content/docs/architecture/overview.md)                   | Package structure, CLI tree, data flows                                                |
| [docs/content/docs/architecture/template-engine.md](./docs/content/docs/architecture/template-engine.md)     | Template engine: delimiters, verbatim copy, conditional files, file permissions, hooks |
| [docs/content/docs/architecture/computed-values.md](./docs/content/docs/architecture/computed-values.md)     | Computed values: post-prompt derived context keys                                      |
| [docs/content/docs/architecture/library-decisions.md](./docs/content/docs/architecture/library-decisions.md) | Library choices and rationale                                                          |
| [docs/operations/release.md](./docs/operations/release.md)                                                   | Release pipeline: GoReleaser, GitHub Releases, Homebrew, CI/CD                         |
