---
title: Storage
weight: 7
next: /docs/architecture
---

Templates are stored under the platform config directory. The location respects `$XDG_CONFIG_HOME` if set.

## macOS

```text
~/Library/Application Support/specs/
└── templates/
    ├── my-template/
    └── another-template/
```

## Linux

```text
~/.config/specs/
└── templates/
    ├── my-template/
    └── another-template/
```
