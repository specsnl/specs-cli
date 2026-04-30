---
name: executing-commands
description: Rules for executing commands safely inside the project.
applyTo: "**"
---

# Repository Execution Rules

## Execution Model

Use the Taskfile tasks for building and testing. Other commands (e.g. `git`, file manipulation,
installing host tools) may run locally on the host.

| Operation | How to run |
|-----------|-----------|
| Run tests | `task test` |
| Build the binary | `task build` |
| Anything else | Run locally on the host |

Never call `docker` or `docker compose` directly — use the Taskfile tasks above.

To list all available tasks:

```shell
task --list
```

## Container Context

`task test` and `task build` execute inside the `go-builder` Docker Compose service. This
service is a one-off container (`--rm`) under the `build` profile — it does not need to be
started before use.

Do not use `task dc:run:go-builder` unless no suitable task exists for the operation. Prefer
creating or extending a task in `Taskfile.dist.yml` instead.
