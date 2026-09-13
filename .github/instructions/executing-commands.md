---
name: executing-commands
description: Rules for executing commands safely inside the project.
applyTo: "**"
---

# Repository Execution Rules

## Execution Model

Use the Taskfile tasks for building and testing. Other commands (e.g. `git`, file manipulation,
installing host tools) may run locally on the host.

| Operation                           | How to run                |
|-------------------------------------|---------------------------|
| Run tests                           | `task test`               |
| Rewrite the golden files            | `task test:update`        |
| Build the binary                    | `task build`              |
| Check Markdown style                | `task md:check`           |
| Fix Markdown (tables + autofixable) | `task md:fix`             |
| Re-record a documentation GIF       | `task demo:record:<tape>` |
| Anything else                       | Run locally on the host   |

Never call `docker` or `docker compose` directly — use the Taskfile tasks above.

To list all available tasks:

```shell
task --list
```

## Container Context

`task test:update` rewrites `internal/util/output/testdata/*.golden` from what the renderer
produces now. Never run it to make a red test green without reading the diff first — that diff
*is* the rendering change.

`task test` and `task build` execute inside the `go-builder` Docker Compose service. This
service is a one-off container (`--rm`) under the `build` profile — it does not need to be
started before use.

`task demo:record:<tape>` re-records `docs/demo/<tape>.tape` into `docs/static/demo/<tape>.gif`
inside the `vhs` Docker Compose service under the `demo` profile. It needs network access and
the Docker socket, and it always dirties the working tree — the same tape never produces
identical bytes twice, so only re-record when the recorded output actually changed. See
[Demo Recordings](../../docs/content/docs/architecture/demo.md).

`task md:check` and `task md:fix` run `markdownlint-cli2` (and, for fixes,
`markdown-table-formatter`) inside the `node` Docker Compose service under the `markdown`
profile. The same checks run in CI via `.github/workflows/md.yml`.

Do not use `task dc:run:go-builder` unless no suitable task exists for the operation. Prefer
creating or extending a task in `Taskfile.dist.yml` instead.
