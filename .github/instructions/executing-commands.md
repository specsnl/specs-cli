---
name: executing-commands
description: Rules for executing commands safely inside the project.
---

# Repository Execution Rules

## Execution Model

All project commands MUST be executed via the Taskfile. Agents MUST NOT call Docker Compose commands directly.

Never run directly on the host (outside `task`):

- go
- docker compose
- docker

Always use:

```shell
task <task-name>
```

To list all available tasks, use:

```shell
task --list
```

If a task does not exist:

1. Inspect the [Taskfile](./Taskfile.dist.yml).
2. Prefer creating or extending a task.
3. As a temporary fallback, use `task dc:run:go-builder -- <command>` unless another service is explicitly required. This still executes via the Taskfile.

## Container Context

The default execution service is `go-builder`.

All standard development commands run inside the Docker Compose service `go-builder`. It is a one-off service (run with `--rm`) under the `build` profile — it does not need to be started before use.

Only use another service if:

- the user explicitly instructs it, or
- the command explicitly references that service.

To open an interactive shell in the container:

```shell
task dc:shell
```

To build the Docker images:

```shell
task dc:build
```

## Examples

Build the binary:

```shell
task build
```

Run a one-off Go command inside the container:

```shell
task dc:run:go-builder -- go test ./...
```

Run tests:

```shell
task test
```

```shell
task build
```
