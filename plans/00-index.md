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
| [11-computed-values.md](./11-computed-values.md) | Computed values: post-prompt derived context keys |

## Implementation

| File | Description |
|------|-------------|
| [09-implementation-plan.md](./09-implementation-plan.md) | Phased overview with package reference |
| [implementation/phase1.md](./implementation/phase1.md) | Phase 1 — Project skeleton (Cobra root, version command) |
| [implementation/phase2.md](./implementation/phase2.md) | Phase 2 — Config & output infrastructure (XDG, lipgloss) |
| [implementation/phase3.md](./implementation/phase3.md) | Phase 3 — Template engine (YAML context, render pipeline) |
| [implementation/phase4.md](./implementation/phase4.md) | Phase 4 — Hooks (pre/post-use, inline yaml + hooks/ dir) |
| [implementation/phase5.md](./implementation/phase5.md) | Phase 5 — Git & host utilities (clone, HTTPS + SSH source parsing, auth) |
| [implementation/phase6.md](./implementation/phase6.md) | Phase 6 — Registry commands (init, list, save, download, validate, rename, delete) |
| [implementation/phase7.md](./implementation/phase7.md) | Phase 7 — `specs template use` (huh prompts, --values/--arg, hooks) |
| [implementation/phase8.md](./implementation/phase8.md) | Phase 8 — `specs use` (one-step: clone/copy → execute → discard) |
| [implementation/phase9.md](./implementation/phase9.md) | Phase 9 — Conditional variable prompting (AST analysis, skip unused variables) |

## Operations

| File | Description |
|------|-------------|
| [10-release-plan.md](./10-release-plan.md) | Release pipeline: GoReleaser, GitHub Releases, Homebrew, CI/CD workflows |
