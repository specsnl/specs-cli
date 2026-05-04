---
title: Quick Start
weight: 2
next: /docs/commands
---

## One-off usage

Use a template directly without registering it first:

```sh
specs use github:specsnl/my-template ./my-project
```

## Registry-based usage

Register a template for repeated use:

```sh
specs template download github:specsnl/my-template my-template
specs template use my-template ./my-project
```
