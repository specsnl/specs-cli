# Specs CLI — Plans

This directory contains the architectural documentation and planning documents for the specs CLI.

## Architecture & decisions

| File | Description |
|------|-------------|
| [01-current-architecture.md](./01-current-architecture.md) | Overview of the current boilr codebase |
| [02-cli-commands.md](./02-cli-commands.md) | CLI commands in depth |
| [03-template-engine.md](./03-template-engine.md) | Template processing engine |
| [04-packages.md](./04-packages.md) | Supporting packages reference |
| [05-library-decisions.md](./05-library-decisions.md) | v2 library choices and rationale |
| [06-template-engine-v2.md](./06-template-engine-v2.md) | Template engine decisions: delimiters and verbatim copy |
| [07-issues-and-prs.md](./07-issues-and-prs.md) | Review of open issues and PRs from both repos |
| [08-v2-architecture.md](./08-v2-architecture.md) | Overall v2 architecture, package structure, data flows |

## Implementation

| File | Description |
|------|-------------|
| [09-implementation-plan.md](./09-implementation-plan.md) | Phased overview with package reference |
| [implementation/phase1.md](./implementation/phase1.md) | Phase 1 — Project skeleton (Cobra root, version command) |
| [implementation/phase2.md](./implementation/phase2.md) | Phase 2 — Config & output infrastructure (XDG, lipgloss) |
| [implementation/phase3.md](./implementation/phase3.md) | Phase 3 — Template engine (YAML context, render pipeline) |

## Operations

| File | Description |
|------|-------------|
| [10-release-plan.md](./10-release-plan.md) | Release pipeline: GoReleaser, GitHub Releases, Homebrew, CI/CD workflows |
